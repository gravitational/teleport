---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: implemented
---

# RFD 24 - DynamoDB Audit Event Overflow Handling

## What

This RFD was sparked from a discussion about event storage, scaling and DynamoDB between me, Russell and Sasha.

Rework the existing read-only global secondary index view to partition on the day of the event instead of the hard coded string `default`.

## Why

DynamoDB has a limit of 10 GB on partitions which are it's internal storage unit for key and index setups similar to ours. Our primary table index partitions on the session ID which is ideal to spread events across partitions so that 10 GB limit is never reached.

We also happen to maintain a DynamoDB Global Secondary Index which acts as a read only materialized view of table. This index does not partition on the session ID but on the namespace field which is the hardcoded string `default`. This means that the Global Secondary Index has a singular partition which is approaching 10 GB on production deployments. When the 10 GB limit is reached, the index will stop synchronizing data from the main table and no new events can be read.

Another concern is that allowing a single partition to contain too many events may impact search times due to the internal B-Tree index growing very deep.

## Details

Currently we do not store a suitable field on audit event entries in Dynamo to partition on. This means that we need to add a new field storing the date of the event in the string format `yyyy-mm-dd`. I propose to create a new Global Secondary index that is identical to existing one except it shall partition on the date key instead of the namespace key. A scheme like this will prevent size blowup on a single partition and instead spread long term data over multiple partitions.

Searching over this new Global Secondary Index is trival since the partition keys are simply a set of dates which can be generated easily from the start and end timestamps provided internally. We generate a set of partition keys to search over and include it in the query.

Since the new date field will not exist on all past events on existing deployments by default, the Teleport auth server will need to go back and retroactively calculate and add this field to all past events. This will be done as a once-off background task created when the DynamoDB backend is created. The consequence of this is that past events will not be visible or searchable until this field as been added but due to the background process they will appear quickly again.

The last thing that will be done is removing the old Global Secondary Index. After this step, the transition is complete.
