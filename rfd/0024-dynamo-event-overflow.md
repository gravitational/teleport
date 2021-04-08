---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 24 - DynamoDB Audit Event Overflow Handling

## What

This RFD was sparked from a discussion about event storage, scaling and DynamoDB between me, Russell and Sasha.

*Primary partition* in this RFD refers to the partition we currently store all events on using a fixed partition key.

Create a new background task when DynamoDB is used for audit event storage that prevents the space consumed by the primary partition to exceed 10 GB by periodically moving the oldest events in the primary partition to date specific partitions until the estimated target space utilization of roughly 9 GB is achieved in the primary partition. This effectively makes the primary partition that is currently used for all events into a sliding window of the 25000 of the newest audit events if we assume that each event has the maximum size of 400 KB.

## Why

DynamoDB imposes a limit on how large an individual partition can be at approximately 10 GB which some deployments are rapidly approaching. There is also the concern of the primary and secondary indexes growing very deep which can increases lookup time and how difficult the data is to work with if exported. This strategy limits the maximum primary partition size to 25000 events.

This strategy preserves events in Dynamo without discarding old ones which I believe is what is naturally expected and may be important for compliance reasons.

## Details

The background service will be implemented as a goroutine started when a DynamoDB event storage backend is created. It will periodically query DynamoDB and estimate how large the primary partition is and if the estimated size exceeds 9 GB it will repeatedly query the oldest events in the partition and attempt to move them to new partitions of the format `yyyy-mm-dd` until the estimated size of the primary partition reaches 9 GB. These per-day partitions are intended as archival partitions and without a local secondary index on them they can scale beyond the 10 GB limit that is imposed on the primary partition.

Since DynamoDB does not allow you to query the combined size of all entries with a certain partition key, we can utilize on the monotonic sort key to determine how many entries we have in the primary partition and can then calculate the maximum possible size of the partition by multiplying with 400 KB which is the maximum item size. This calculated size is what the background task will use to determine when to move old audit events to per-day partitions.

### Example logic flows for background task

#### Example 1

The task queries the table with the primary partition key for the least and greatest event IDs. It then performs the calculation `greatestID - leastID` and determines that the primary partition contains 12000 items.

This number is multiplied by the maximum event size of 400 KB and we can thus assume that the partition size is lesser or equal to `12000 * 400 KB = 4.8 GB`. This number is under our maximum target of 9 GB. The task puts itself to sleep for five minutes and then performs the same calculation again, moving events if needed.

#### Example 2

The task queries the table with the primary partition key for the least and greatest event IDs. It then performs the calculation `greatestID - leastID` and determines that the primary partition contains 12000 entries.

This number is multiplied by the maximum event size of 400 KB and we can thus assume that the partition size is lesser or equal to `24000 * 400 KB = 9.6 GB`. This number is above our target maximum 9 GB. We are over target by `600 MB / 400 KB = 1500 events` if we assume each event is of maximum item size.

The task queries the first 1500 events ordered by sort key ascending and moves these into other partitions depending on their day of occurence according to the partition key format detailed above.
