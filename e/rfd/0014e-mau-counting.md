---
author: Bartosz Leper (bartosz.leper@goteleport.com)
state: draft
---

# RFD 0014e - Counting Monthly Active Users

## Required Approvers

- Engineering: @espadolini
- Product: (@xinding33 || @klizhentas)

## What

Provide a uniform way to count monthly active users (MAU) for all Teleport billing plans and deployment types.

## Why

We would like to provide a clear and predictable experience for all Teleport users and provide a payment mechanism that would be both easy to implement and fair to our customers.

## Details

### Current State

Currently, Teleport clusters report user activity by either aggregating or streaming reports about each user. The users are identified by user name and cluster ID. For privacy reasons, we anonymize usernames with HMAC-SHA256, with the encryption key set to the cluster ID. This means that our customers can be double-charged for user activity when a given user performs activity on more than one cluster, since that user's activity records will bear different hashes for different clusters.

### Solution

Instead of using cluster ID as an HMAC key, we will derive the HMAC key from an anonymization key stored in a separate custom section of the customer's license file. This way, the users' identities will not be tied to a particular cluster, but to the entire customer's infrastructure. The license file uniquely identifies the customer's account, and using the following design, the anonymization key can be known only to the customer, without exposing it to Teleport's infrastructure.

To achieve this, we will change the license downloading procedure. The browser will fetch the license PEM file, just as it does currently; after that, it will generate an anonymization key on the client side and append it to the license file as a custom PEM section before saving the file on the user's disk.

After the customer distributes the license file, all clusters will share the same license key file, and by implication, the same anonymization key. Go's [standard PEM encoding library](https://pkg.go.dev/encoding/pem) is flexible enough to support this case.

Applications that perform anonymization on the client side will retrieve the anonymization key from the auth server after signing in.

#### Tradeoffs

One caveat here is that until now, the license itself was "stable" — it could be downloaded multiple times, and its contents would remain the same. After this change, since the anonymization key will never be exposed to the signing authority (Teleport), this process should ideally be done only once. Every redeployment of the license, as well as accidental deployment of multiple different license files, will lead to miscalculations in billing. Because of that, we need to change the UI to reflect that fact and make sure that the customer is aware of the risk involved in mishandling the license file.

We may be able to circumvent this by requiring or otherwise strongly suggesting to provide an old license file to download a new one. The exact UX here will be designed as a follow-up step.

Another implication is that changing any of the license parameters that leads to issuing another license also changes the data anonymization key. It means that such an event must effectively restart the billing cycle, regardless of whether this change would enforce it anyway or not.

We also compromise on accuracy of counting users. While in case of SSO users, there is no way for multiple users to share the same user name, it may happen in case of cluster-specific username pools. In case when two different users share the same username on two different clusters, they will still be counted as one. However, we predict that this corner case will not have significant effect on our revenue. At the same time, it's better to err on the side of customer — it's better to charge too little for users who are unique but appear non-unique, than charge too much for the opposite case.

#### Anonymizing other data

Teleport clusters are currently anonymizing more than just user names. Other data, such as resource names, are anonymized as well. All anonymized strings that are, in fact, cluster-scoped, like the aforementioned resource names, will need to be kept in the cluster scope. We will achieve this by combining both customer-scoped anonymization key _and_ cluster ID for anonymizing these strings.

#### Release

The new rules will need to be applied conditionally. The target audience of the feature would be customers on the upcoming enterprise usage-based plan. However, we also need to be able to activate this calculation scheme on the boundary of billing cycle for each affected customer. The billing pipeline will need to tell the difference between data reported using the old and new scheme and make sure that only old or new scheme is accounted for in the previous or next billing cycle, respectively.

We propose that any server that encounters a license file with the additional PEM section for the anonymization key applied the key and also add a new `event_format` field to the event metadata, setting it to 1. This will serve as a notification that the events need to be interpreted slightly differently. Using an integer instead of a boolean flag should enable us to further tweak this logic if needed in future.

Releasing this change _before_ we allow customers to enroll into an enterprise usage-based plan will allow us to avoid having to force existing customers to download and distribute an updated license.

### Alternatives considered

An obvious simpler alternative of this solution would be to use account ID for anonymization. However, since Teleport has access to the account ID itself, we could deanonymize the usage data by performing a brute-force search over the list of usernames.

We could generate the license using a command-line utility or force the customer to create the CSR manually. This, however, would mean additional friction for the customers.

We could also use an anonymization key separate from the license, generated either by the UI or a command-line utility, and expect the customer to distribute it properly. However, this would mean both additional friction and another configuration dimension that may increase overall complexity.

We could use the license's private key, moving the private key generation from the server side to the client side. However, this would require us to expose the license private key to client-side applications such as Teleport Connect. However, distributing the private key this way increases the risk of license spoofing in case if this key is leaked outside the customer's control.