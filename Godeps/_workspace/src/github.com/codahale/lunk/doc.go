// Package lunk provides a set of tools for structured logging in the style of
// Google's Dapper or Twitter's Zipkin.
//
// The Context Of Structured Logging
//
// When we consider a complex event in a distributed system, we're actually
// considering a partially-ordered tree of events from various services,
// libraries, and modules.
//
// Consider a user-initiated web request. Their browser sends an HTTP request to
// an edge server, which extracts the credentials (e.g., OAuth token) and
// authenticates the request by communicating with an internal authentication
// service, which returns a signed set of internal credentials (e.g., signed
// user ID). The edge web server then proxies the request to a cluster of web
// servers, each running a PHP application. The PHP application loads some data
// from several databases, places the user in a number of treatment groups for
// running A/B experiments, writes some data to a Dynamo-style distributed
// database, and returns an HTML response. The edge server receives this
// response and proxies it to the user's browser.
//
// In this scenario we have a number of infrastructure-specific events:
//
//     1.  The edge server handled a request, which took 142ms and whose
//         response had a status of "200 OK".
//     2.  The edge server sent a request to the authentication service, which
//         took 5ms to handle and identified the principal as user 14002.
//     3.  The authentication service handled a request, which took 4ms to
//         handle and was served entirely from memory.
//     4.  The edge server proxied a request to the app cluster, which took
//         132ms and whose response had a status of "200 OK".
//     5.  The app load balancer handled a request, which took 131ms and whose
//         response had a status of "200 OK".
//     6.  The app load balancer proxied a request to the app, which took 130ms
//         and was handled by app server 10.
//     7.  The app handled a request, which took 129ms, and was handled by
//         PhotoController.
//     8.  The app sent a query to database A, which took 1ms.
//     9.  The app sent a query to database B, which took 53ms.
//     10. The app rendered template "photo.tpl", which took 4ms.
//     11. The app wrote a value to the distributed database, which took 10ms.
//     12. The distributed database handled the write locally on one node, and
//         proxied it to two others, which took 9ms.
//     13. Those distributed database nodes concurrently handled the write
//         locally, which took 4ms and 8ms.
//
// This scenario also involves a number of events which have little to do with
// the infrastructure, but are still critical information for the business the
// system supports:
//
//     14. The app gave the user the control treatment for experiment 15
//         ("Really Big Buttons v2").
//     15. The app gave the user the experimental treatment for experiment 54
//         ("More Yelling v1").
//     16. User 14002 viewed photo 1819 ("rude-puppy.gif").
//
// There are a number of different teams all trying to monitor and improve
// aspects of this system. Operational staff need to know if a particular host
// or service is experiencing a latency spike or drop in throughput. Development
// staff need to know if their application's response times have gone down as a
// result of a recent deploy. Customer support staff need to know if the system
// is operating nominally as a whole, and for customers in particular. Product
// designers and managers need to know the effect of an A/B test on user
// behavior. But the fact that these teams will be consuming the data in
// different ways for different purposes does mean that they are working on
// different systems.
//
// In order to instrument the various components of the system, we need a common
// data model.
//
// Trees Of Events
//
// We adopt Dapper's notion of a tree to mean a partially-ordered tree of events
// from a distributed system. A tree in Lunk is identified by its root ID, which
// is the unique ID of its root event. All events in a common tree share a root
// ID.  In our photo example, we would assign a unique root ID as soon as the
// edge server received the request.
//
// Events inside a tree are causally ordered: each event has a unique ID, and an
// optional parent ID. By passing the IDs across systems, we establish causal
// ordering between events. In our photo example, the two database queries from
// the app would share the same parent ID--the ID of the event corresponding to
// the app handling the request which caused those queries.
//
// Each event has a schema of properties, which allow us to record specific
// pieces of information about each event. For HTTP requests, we can record the
// method, the request URI, the elapsed time to handle the request, etc.
//
// Event Aggregation
//
// Lunk is agnostic in terms of aggregation technologies, but two use cases seem
// clear: real-time process monitoring and offline causational analysis.
//
// For real-time process monitoring, events can be streamed to a aggregation
// service like Riemann (http://riemann.io) or Storm
// (http://storm.incubator.apache.org), which can calculate process statistics
// (e.g., the 95th percentile latency for the edge server responses) in
// real-time. This allows for adaptive monitoring of all services, with the
// option of including example root IDs in the alerts (e.g., 95th percentile
// latency is over 300ms, mostly as a result of requests like those in tree
// XXXXX).
//
// For offline causational analysis, events can be written in batches to batch
// processing systems like Hadoop or OLAP databases like Vertica. These
// aggregates can be queried to answer questions traditionally reserved for A/B
// testing systems. "Did users who were show the new navbar view more photos?"
// "Did the new image optimization algorithm we enabled for 1% of views run
// faster? Did it produce smaller images? Did it have any effect on user
// engagement?" "Did any services have increased exception rates after any
// recent deploys?" &tc &tc
//
// Observing Specific Events
//
// By capturing the root ID of a particular web request, we can assemble a
// partially-ordered tree of events which were involved in the handling of that
// request. All events with a common root ID are in a common tree, which allows
// for O(M) retrieval for a tree of M events.
//
// Sending And Receiving HTTP Requests
//
// To send a request with a root ID and a parent ID, use the Event-ID HTTP
// header:
//
//     GET /woo HTTP/1.1
//     Accept: application/json
//     Event-ID: d6cb1d852bbf32b6/6eeee64a8ef56225
//
// The header value is simply the root ID and event ID, hex-encoded and
// separated with a slash. If the event has a parent ID, that may be included as
// an optional third parameter.  A server that receives a request with this
// header can use this to properly parent its own events.
//
// Event Properties
//
// Each event has a set of named properties, the keys and values of which are
// strings. This allows aggregation layers to take advantage of simplifying
// assumptions and either store events in normalized form (with event data
// separate from property data) or in denormalized form (essentially
// pre-materializing an outer join of the normalized relations). Durations are
// always recorded as fractional milliseconds.
//
// Log Formats
//
// Lunk currently provides two formats for log entries: text and
// JSON. Text-based logs encode each entry as a single line of text, using
// key="value" formatting for all properties. Event property keys are scoped to
// avoid collisions. JSON logs encode each entry as a single JSON object.
package lunk
