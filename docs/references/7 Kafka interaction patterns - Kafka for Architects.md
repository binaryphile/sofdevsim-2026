### This chapter covers

-   Patterns for applying Kafka in microservices
-   Implementing a data mesh architecture with Kafka
-   Integrating data using Kafka Connect
-   An introduction to Kafka transactions

Now that you have seen some real-world use cases, let’s look at how Kafka fits into architectural patterns, so you can choose it deliberately, not by default. To make these choices, it’s important to know where Kafka excels in microservices, including smart endpoints, data mesh, CQRS, and event stores, and where it’s a poor match (request-response).

Then there’s the issue of how to move data in and out—for this we can turn to Kafka Connect, which we met briefly in chapter 5. We’ll explore in detail how this product can be effectively used for data integration within an enterprise environment, examining both its potential and common challenges.

Finally, we need to protect our system from the risk of undetected data loss—always a concern in asynchronous message transfer—using Kafka’s guarantees. How do we ensure durability (no data loss), exactly-once processing, and ordering, and what do these semantics actually guarantee?

This chapter helps turn patterns into practical choices you can defend in production, backed by clear checklists and tradeoffs.

## 7.1 Using Kafka in microservices

Let’s start by exploring architectural patterns commonly used in microservices and evaluating how effectively Kafka can address them. While Kafka excels in many scenarios, it may not be the ideal solution for certain patterns, and we will highlight these limitations.

### 7.1.1 Smart endpoints and dumb pipes

In a Kafka-based architecture, the _smart endpoints, dumb pipes_ pattern emerges naturally as the only viable approach. Since Kafka brokers merely transport messages and do not interpret them, all processing logic must reside at the endpoints. This makes the pipes truly “dumb,” serving purely as communication channels, fully decoupled from any processing logic. An example of such a decentralized architecture is illustrated in figure 7.1.

##### Figure 7.1 All the logic resides on the smart endpoints; the pipes serve only for transport.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image001.png)

This contrasts sharply with the _enterprise service bus_ (ESB) pattern, where microservices communicate through middleware that not only transports messages but also performs additional tasks. These tasks may include data validation, enforcing authorization rules, encryption, dynamic routing, and auditing, as shown in figure 7.2.

##### Figure 7.2 Integration logic is handled by smart middleware (the enterprise service bus).

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image002.png)

Modern architectural approaches often consider ESB an antipattern for two primary reasons:

-   Centralizing logic in the ESB creates a bottleneck, slowing down development.
-   Deploying the ESB as a monolithic product introduces a single point of failure—even if it’s clustered, it remains a logically unified component in the architecture.

If an ESB centralizes too much logic and becomes an antipattern, this raises an important question: What about integration logic that isn’t directly related to business functionality? For instance, data transformation might be considered business logic unique to each endpoint, but should we also implement concerns like encryption or auditing within each service? These cross-cutting concerns are typically integration-related, and need to be standardized across the enterprise, so they should be configurable rather than hardcoded at each endpoint.

The solution to this challenge lies in a service mesh. A _service mesh_ introduces an infrastructure layer designed to streamline service-to-service communication in a microservice architecture. In this approach, a microservice is effectively divided into two components:

-   The business logic component, which implements the actual functionality of the service.
-   The system component, which handles cross-cutting concerns such as encryption, auditing, retries, observability, and other integration-related responsibilities not directly tied to the service’s core business logic.

The system component is designed to be reusable across multiple microservices and is configurable to suit specific needs.

A service mesh leverages the _sidecar_ pattern, where each microservice consists of two containers: one for the business logic, and another, the sidecar, for managing integration logic. All network traffic to the business logic container is routed through the sidecar proxy. The sidecar handles system-level concerns, such as encryption, retries, load balancing, and others, before forwarding requests to the business logic container. This separation offloads integration and infrastructure responsibilities from the business logic, enabling cleaner, more focused code.

An example of a service mesh architecture is illustrated in figure 7.3. These sidecar proxies are typically off-the-shelf components that perform predefined tasks based on configuration. One of the most widely adopted implementations of this pattern is Envoy ([www.envoyproxy.io](https://www.envoyproxy.io/)), a high-performance, open source proxy that powers many service mesh frameworks.

##### Figure 7.3 An example of a service mesh architecture. Business logic and integration logic are separated into different containers. All network traffic flows through the sidecar component, which acts as a proxy to the component’s pure business logic.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image003.png)

How are these sidecars configured? When integration logic is managed by a product like a service bus, these products typically provide tools such as a service catalog. A service catalog offers visibility into all available services and allows users to configure complex integration logic in a centralized manner. In a service mesh, this functionality is handled by a dedicated component known as the _service mesh control plane_, which is shown in figure 7.4.

##### Figure 7.4 Configuring sidecar containers through the control plane

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image004.png)

The control plane is a critical concept in a service mesh architecture, responsible for the centralized management of all sidecar proxies. It provides the functionality to configure, deploy, and manage the behavior of the network across the entire mesh. The control plane enables you to define traffic policies, security configurations, load balancing rules, and observability features, ensuring consistent behavior across participants.

In practice, the control plane is often implemented as a product or a set of components—examples include Istio’s control plane ([https://istio.io](https://istio.io/)) and Linkerd’s control plane ([https://linkerd.io](https://linkerd.io/))—that provide a user interface or API for configuring these rules. This centralization ensures that service-to-service communication is managed in a scalable and dynamic way, without embedding complex integration logic directly into the services themselves.

But how do services that communicate through Kafka relate to a service mesh? In earlier chapters, you saw that much of the Kafka-related integration logic is handled by client libraries. For example, functionality like batching messages on the producer side or retrying message delivery if an acknowledgment is not received are implemented within the Kafka client library, not directly in the services themselves. This raises the question: Should this functionality be moved to the sidecar container instead of being handled by the client library?

This is where the overlap between Kafka client libraries and sidecar proxies becomes evident, making Kafka-enabled services less compatible with traditional service mesh designs. The community is actively discussing and exploring ways to make these concepts compatible. Experimental efforts, such as the Kafka mesh filter, aim to bring Kafka communication into the service mesh paradigm. However, this remains a hot topic at the cutting edge of microservices architecture, with no definitive solution yet widely adopted.

### 7.1.2 Request-response pattern

So far in this book, all our examples have been about implementing the publish-subscribe pattern, where producers are unaware of consumers and multiple consumer groups are allowed to read the same message. But can we also use Kafka for a _request-response_ pattern, where one service sends a request and expects a reply to come back to the same service instance? In a distributed architecture, this behavior introduces several challenges:

-   The service needs a way to pair requests and responses. This is typically achieved by setting a special header, such as a correlation ID, so that both the request and the corresponding response carry the same value of the correlation ID.
-   If multiple instances of a service are sending requests, the response must return to the instance that sent the request.

Some frameworks, like Spring Kafka ([https://spring.io/projects/spring-kafka](https://spring.io/projects/spring-kafka)), implement this pattern, allowing users to avoid dealing with the complexity of the implementation details. An example of this flow is shown in figure 7.5.

##### Figure 7.5 Both services participating in the request-response pattern have a producer and consumer container. The request and response are matched using the correlation ID, specified in the message header.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image005.png)

When a request is sent, a special CORRELATION\_ID header is added, and the service expects it to be echoed back in the response. The trickiest part of this pattern is ensuring that the response is delivered to the correct instance of the service.

In a typical consumer group scenario, when a consumer instance fails, its partitions are reassigned to healthy consumers. However, in the request-response pattern, the consumer instance that takes over the partition after a failure cannot process the response correctly because it lacks the request context. This situation is illustrated in figure 7.6.

##### Figure 7.6 The availability of RequestResponseService is problematic, as another instance does not have the original request context.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image006.png)

There are several approaches to resolving this issue, and different client frameworks implement various strategies:

-   _Manual partition assignment_—In this approach, partitions for the consumers in RequestResponseService are assigned manually. When a request is sent, the headers include information about the response topic and partition where the response should be delivered. The assignment of the consumer instance to the partition is done manually, and if the consumer instance fails, no automatic recovery happens. Custom logic needs to handle any failures in consumers reading replies. An example of this approach is shown in figure 7.7.

##### Figure 7.7 Each message carries a correlation ID, a topic, and a partition so that ResponderService knows where to send the response.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image007.png)

-   _Broadcasting responses to all consumers_—Another approach (shown in figure 7.8) is to simulate broadcasting by having each instance of RequestResponseService use a different consumer group. This ensures that each instance receives all responses, allowing it to select only those that match its own requests and ignore the rest. If an instance fails and is restarted with the same consumer group, it can still read the messages (although it may lose the request context). This approach improves availability, but it may require additional filtering logic.

##### Figure 7.8 Each instance of the service has its own consumer group assigned.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image008.png)

-   _Dedicated reply topics for each instance_—In this approach (shown in figure 7.9), a dedicated reply topic is created for each instance of RequestResponseService. When a request is sent, it specifies the dedicated topic for the instance that initiated the request, ensuring the response is delivered only to that instance. This approach provides isolation but can lead to a large number of topics if many service instances are running. Managing these topics becomes a consideration.

##### Figure 7.9 Each instance of the service has a dedicated topic for the responses.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image009.png)

All of these implementations must address two key challenges:

-   _Recovery when_ RequestResponseService _fails_—In the request-response pattern, consumer rebalancing cannot be used across different service instances, so a process for handling recovery is essential. When an instance of the service fails, the request context is lost. This loss must be monitored and reported. One option is to store the request context in external storage, but this obviously impacts performance. The system must ensure that recovery mechanisms are in place for dealing with these failures.
-   _Handling unpaired requests_—Special logic is required to handle cases where a request is sent but the response is never received within a specified timeout. For instance, if no response is received, the client may choose to resend the request. Ensuring idempotence is crucial here, to avoid side effects when resending requests. Alternatively, unpaired requests can be sent to dead-letter topics, where they can be monitored and investigated.

All in all, implementing this pattern in Kafka introduces complexity and is often regarded as an antipattern. Kafka is optimized for asynchronous, broadcast-style communication, where consumers are decoupled from producers, and messages can be processed independently by multiple consumer groups. Attempting to enforce synchronous request-response behavior in Kafka can lead to several drawbacks:

-   Difficulty in handling consumer failures
-   The need for complex correlation and context management
-   Potential performance issues, especially when external storage is used for context tracking

Whether using manual partition assignment, broadcasting responses to all consumers, or dedicated response topics, all request-response patterns in Kafka share a key limitation: Kafka itself provides no guarantee that the responding service will send a response at all. If the responder crashes, times out, or encounters an error before replying, the requesting client will never receive a response—unless additional logic is implemented. To address this, the client application must handle timeouts, correlation, and potential failure scenarios independently. Kafka’s log-based architecture offers durable messaging, but it does not enforce or monitor the completion of response flows, making it essential to build reliability mechanisms into the surrounding application logic.

In many cases, other messaging systems like RabbitMQ or protocols like gRPC, or even simple HTTP—might be better suited for strict request-response interactions, as they are designed to handle synchronous, point-to-point communication natively.

### 7.1.3 CQRS pattern

In the previous chapters, when we introduced the ProfileService in our examples, we treated it as the microservice responsible for all profile updates: it handled write operations, stored records in its own database, and published events whenever profile data changed. In this architecture, the ProfileService is the single point for updating profile data. But what about reading data? Should the ProfileService expose a synchronous REST service for reading profile information? Or, perhaps that’s the responsibility of another microservice? Perhaps any other microservice interested in profile data should simply listen for events and build its own view of profiles, tailored to its needs? What are the criteria for making these decisions?

The pattern where write operations and read queries are separated is known as _Command Query Responsibility Segregation_ (CQRS). In this context, a command refers to an operation that changes the state (e.g., creating or updating data), while a query refers to an operation that retrieves data.

This separation is illustrated in figure 7.10 where two different object models exist: one for updating data and one for reading it. These models may differ conceptually or physically and may even rely on distinct persistence mechanisms. However, CQRS doesn’t necessarily mean that these models must belong to separate microservices. CQRS can be applied at different levels, and whether you need to split a service depends on factors such as the complexity of your application, performance needs, and architectural considerations.

##### Figure 7.10 Different object models are responsible for writing and reading the data.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image010.png)

For services that primarily handle CRUD operations, CQRS can introduce unnecessary overhead. In the following scenarios, CQRS can definitely add value:

-   When the model for querying is much more complex than the model for updating data
-   When read operations are far more frequent than write operations, and the reading logic requires different scaling strategies

The CQRS pattern fits perfectly into the Kafka ecosystem and is often combined with event sourcing. In figure 7.11, you’ll see a microservice responsible for command logic writing data into its internal storage and emitting events to Kafka.

##### Figure 7.11 Implementing the CQRS pattern with Kafka. Here OrderService is a command, which updates orders, and the other services are query services. Query services may have pretransformed denormalized data.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image011.png)

Various other microservices consume these events, each handling data for reading in different ways. Some services may read data directly from Kafka topics and maintain an in-memory image, while others may store the data in their own internal databases, pre-optimized for various types of queries. Implementing the CQRS pattern with Kafka is an excellent choice when you need to maintain a complete history of all data changes.

While CQRS is widely used in event-driven architectures, it does come with certain drawbacks:

-   _Eventual consistency_—The data in the read models is not updated transactionally but rather with some delay. This means the read views may not always reflect the most current state at any given time.
-   _Data duplication_—To support different models for reading and writing, data is often duplicated, which can lead to increased storage costs.
-   _Model synchronization_—When business requirements change, any updates to the system must be applied to both the write model and all corresponding read models, which can increase the complexity of keeping the system synchronized.

### 7.1.4 Event sourcing with snapshotting

The idea of restoring state from events works conceptually but raises important concerns about performance, particularly when a microservice needs to restore its state during startup. Replaying every event to rebuild the state can be resource-intensive and time-consuming. This challenge is addressed by the pattern known as _event sourcing with snapshotting_. The same concept appears in Kafka’s compacted topics, where for each key, only the most recent message is guaranteed to be accessible.

However, if a full log of events is required for auditing or complete history tracking, compacted topics are not suitable. Instead, a process can periodically capture the current state of the system (a snapshot) and store it in a more efficient, external storage solution. This snapshot can be taken on a regular schedule (e.g., once per day), or after a certain number of events (e.g., every 100,000 events), or it can be triggered by a business event (e.g., “end-of-day” in a banking system).

Figure 7.12 illustrates the general idea, where snapshots are stored in external storage like MongoDB. Along with the snapshot data, we also store the corresponding position in Kafka, identified as <topic, partition, offset>. This enables the system to know exactly where the snapshot was taken within the event stream.

##### Figure 7.12 The state can be periodically uploaded to the database and then restored from there.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image012.png)

When the consumer microservice starts, it first retrieves the latest snapshot from MongoDB. It then replays only the events that occurred after the snapshot was taken, significantly reducing the time and effort needed to restore the state.

While snapshotting improves performance during state restoration, it comes with certain drawbacks. Snapshot creation adds overhead, as it requires additional processing and storage. Additionally, managing snapshot consistency across distributed systems can be complex, especially if there are concurrent updates or failures during the snapshot process.

### 7.1.5 Having “hot” and “cold” data

Storing messages on disk can become a challenge when you need to retain a large volume of data for an extended period. Additionally, access patterns vary significantly: recent (“hot”) data is accessed much more frequently than older (“cold”) data. The concept of _tiered storage_ addresses this by segregating hot and cold data into different storage tiers, optimizing both performance and cost. This feature is available in Kafka starting from version 3.6.0.

As illustrated in figure 7.13, hot data resides on the local disks of the Kafka broker, ensuring low-latency access, while cold data is periodically offloaded to cheaper, cloud-based storage. In practice, this allows Kafka to retain data for much longer durations—potentially indefinitely—without being constrained by the size of local disk storage.

##### Figure 7.13 When the tiered storage feature is enabled, the cold data is moved periodically to cheaper, cloud-based storage.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image013.png)

The configuration allows you to specify a remote storage location where older, rolled-over segments are moved. Current implementations support Amazon S3, Azure Blob Storage, and Google Cloud Storage, while also offering the flexibility to integrate custom storage solutions, whether in the cloud or on-premises. To enable tiered storage, the remote.log.storage.system.enable property must be set to true at the cluster level. It is also possible to set it at the topic level by setting the remote.storage.enable property.

From a consumer’s perspective, access to the data remains completely transparent. Consumers do not need to be aware of whether the data resides on local disk or in remote storage. They can perform normal operations as if all data is local, and operations that do not require access to remote storage will function normally, even if remote storage is temporarily unavailable. In cases where consumers attempt to access data stored remotely and the remote storage is inaccessible, Kafka provides graceful error handling with meaningful error messages.

When data is offloaded to remote storage, not only are the data segments moved, but so is the associated metadata. Kafka does not clean up local logs until these segments are successfully copied to the remote storage, even if their local retention time or size limit has been reached. This ensures consistency between local and remote data storage.

Additionally, tiered storage alters the behavior of follower replicas. Traditionally, followers would continuously copy data from the leader to keep their replicas in sync. With tiered storage enabled, the role of the followers changes:

-   Followers need to copy local data from the leader.
-   Followers must also copy and maintain remote log metadata from the remote storage manager to construct any auxiliary state they might need for full functionality.

In general, hot storage should retain the actively accessed working set and the window required for recovery and backfills; older data belongs in the cold tier. In Apache Kafka’s tiered storage, uploads are not scheduled manually. The Remote Log Manager automatically offloads closed (inactive) segments—including their index files—to remote storage after segment roll; you influence the cadence indirectly via segment.bytes, segment.ms, and remote.log.manager.task.interval.ms. To avoid overloading infrastructure, apply remote-storage quotas (e.g., remote.log.manager.copy.max.bytes.per.second and related settings).

If you need tiering for compacted topics, Confluent Tiered Storage supports it, starting with Confluent Platform 7.6.

## 7.2 Decentralizing analytical data with a data mesh

So far, we’ve discussed operational data—generated by microservices through their day-to-day activities, and continuously updated to reflect the current state of the business. However, we haven’t addressed analytical data, which is typically derived from operational data and used for long-term analysis. Traditionally, analytical data is aggregated from multiple sources into a central repository. This repository can take the form of a data warehouse, which stores structured data optimized for business intelligence and historical reporting, or a data lake, which stores raw data allowing flexible processing across various formats.

While centralizing data enables comprehensive analysis pipelines across all available data, it also introduces significant challenges. Analytical teams often find it overwhelming to manage and build logic across all domain data, struggling to keep up with evolving application models. Can we apply the same principles of microservice architecture—breaking tasks into smaller, manageable components?

This is where the data mesh ([www.datamesh-architecture.com](https://www.datamesh-architecture.com/)) paradigm comes in. Data mesh advocates for decentralizing analytical data based on four core principles:

-   _Domain ownership_—Responsibility shifts from a central analytics team to domain teams that have the best understanding of their data.
-   _Data as a product_—Data is treated as a product, with clear contracts and a guarantee of quality, making it reliable and consumable by others.
-   _Federated governance_—Although it’s decentralized, data must still comply with enterprise-wide standards related to business, legal, and security requirements. This is enforced through governance policies applied consistently across all data products.
-   _Self-serve platform_—There must be an automated, consistent way to inform data consumers about planned changes and provide them with the tools to access and use data without heavy dependencies on centralized teams.

The data mesh is a conceptual framework and does not prescribe a specific technology for implementation. However, we can explore how it can be effectively implemented using Apache Kafka.

### 7.2.1 Domain ownership

Exposing data from Kafka aligns strongly with the data mesh principle of domain ownership. As discussed in chapter 6, in event-driven systems, data ownership resides with the producer. The producer holds the domain knowledge necessary to decide what data to expose and how to structure it. When Kafka is used as the integration layer, domain boundaries are naturally established through topics and schemas. It’s the producer team that manages the structure, schema, and quality of data they publish, ensuring it aligns with the needs and semantics of their specific domain. Kafka’s Schema Registry further supports this by enabling data contracts at the technical level, helping maintain consistency and quality across data consumers.

Challenges arise, however, when use cases require combining data from multiple domains. Meeting such cross-domain requirements calls for well-established collaboration processes between teams to ensure interoperability and alignment without compromising individual domain autonomy.

### 7.2.2 Data as a product

Treating data as a product means data becomes a first-class citizen, with exposed data meeting the standards of a public interface. Kafka naturally supports this approach, because events are viewed as data products with structures defined by data contracts in clear, understandable formats. Schema Registry plays a key role here, enabling teams to define and manage these contracts. As requirements for data contracts evolve, Kafka’s support for schema compatibility allows for a smooth evolution, minimizing disruptions for data consumers.

With data as a product, it’s also essential to provide comprehensive documentation and metadata that help consumers understand the data’s purpose, structure, and context. However, a gap often exists between analytical models and the technical data contracts maintained in Schema Registry. This gap, as discussed in chapter 6, highlights the need for alignment between business-oriented data definitions and the technical schemas that represent them.

### 7.2.3 Federated governance

Enabling autonomous data management is valuable, but all data products within an organization must still comply with legal or industry-mandated policies, such as the following:

-   _Personal information protection_—Ensuring compliance with data privacy regulations, like GDPR or CCPA, by controlling access and anonymizing sensitive data
-   _Standardizing data structures_—Enforcing consistent data formats, such as specifying whether object identifiers must be integers or strings, to improve data compatibility across systems
-   _Logging and data profiling_—Defining guidelines for logging and profiling data to detect and address potential errors proactively
-   _Data access policies_—Setting rules on who can access specific data products to maintain data security and regulatory compliance

While federated governance is achievable with Kafka as the integration layer, the platform alone provides limited direct support for enforcing these policies. Regular audits and reviews are essential for maintaining compliance without compromising the decentralized model, helping to ensure that each domain meets the organization’s governance standards.

### 7.2.4 Self-serve platform

Allowing domain teams to publish their own data significantly accelerates product delivery. However, for effective cross-team collaboration, a self-serve platform is essential. This platform should provide

-   _Capabilities for creating, monitoring, and publishing data products_—Enabling teams to independently manage the full lifecycle of their data products
-   _Data product discovery_—Allowing teams to easily locate and understand data products across the organization
-   _Processes for requesting data product changes_—Providing a streamlined way for consumers to request modifications, fostering responsiveness and adaptability
-   _Automated policy enforcement_—Ensuring that all data products adhere to compliance and governance standards without manual intervention

A platform with these features helps teams work with data products in an automated and consistent manner. Technically, this can be supported through DataOps practices, which are explored further in chapter 10.

It’s worth mentioning that while Kafka provides strong alignment with data mesh principles, it doesn’t need to serve as the system of record. In practice, the producing system remains the source of truth. Kafka’s role is to expose the data produced by these systems through well-defined, governed interfaces—typically in the form of event streams.

## 7.3 Using Kafka Connect

Kafka Connect is a standalone component designed to simplify the integration of external systems (such as databases, filesystems, cloud services, and business applications) with Kafka, requiring minimal to no coding. The goal is to enable developers to transfer data to or from Kafka via a connector that’s configured with a connector configuration file containing all the necessary information for the integration process. In addition to connector configurations, Kafka Connect also relies on plugins—Java libraries that enable specific types of connectors to interact with different technologies.

Kafka Connect offers two types of connector plugins:

-   _Source connectors_—These connectors retrieve data from external systems and publish it to Kafka topics.
-   _Sink connectors_—These connectors take data from Kafka topics and send it to external systems.

This entire process operates in real time, meaning new data in one system is quickly propagated to the other. The speed of this propagation depends on factors such as the configuration of the connector, the performance of the source or sink system, the network bandwidth, and the processing capacity of the Kafka Connect instance.

Kafka Connect is designed to be modular and pluggable, requiring specific plugins to be installed for integration with external systems (illustrated in figure 7.14). For instance, to connect with a relational database, a developer must:

1.  Install a connector plugin for database integration.
2.  Configure the connector with mappings between database tables and Kafka topics.

##### Figure 7.14 Connector plugins must be installed in Kafka Connect to enable specific technologies. Connectors are configured via configuration files that contain instructions on how to pull or push data.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image014.png)

A wide variety of plugins are available, covering most common technologies; some are open source, while others require a commercial license. All plugins are built on the Kafka Connect framework and can be uniformly managed.

Using Kafka Connect offers several key benefits:

-   _Ready-to-use connector plugins_—A marketplace of connector plugins enables rapid integration, significantly reducing time to market.
-   _Customizable API for new connector plugins_—If a connector plugin does not exist or licensing is an issue, developers can create a new connector plugin using the API and framework provided.
-   _Management via REST API_—Kafka Connect includes REST API endpoints for creating, starting, stopping, and managing connectors, offering centralized and streamlined control.
-   _Built-in scalability and reliability_—Kafka Connect is designed to be natively scalable, with automatic failover and recovery capabilities, ensuring reliability in production environments.

### 7.3.1 Kafka Connect at a glance

Figure 7.15 illustrates how Kafka Connect integrates into the Kafka ecosystem.

##### Figure 7.15 How Kafka Connect fits into the Kafka ecosystem

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image015.png)

Suppose we have installed a connector plugin and created a JSON file containing the connector configuration. Next, we can deploy this configuration via the REST API, after which the connector will begin running on the Kafka Connect server. Kafka Connect doesn’t require dedicated storage, such as a database; instead, it uses a dedicated Kafka topic (by default, connect-configs) to store configuration data. Connectors themselves function as clients within the Kafka cluster:

-   _Source connectors_—These connectors pull data from external systems into Kafka, acting as producers. All producer-specific configuration settings, covered in chapter 3, can be applied to source connectors for customization.
-   _Sink connectors_—These connectors pull data from Kafka and send it to external systems, functioning as consumers. They support all standard consumer configuration options for fine-tuning, as discussed in chapter 4. Specifically, each sink connector has its own group.id, which is automatically assigned if not explicitly set, and is used to manage partition assignments and offset tracking like in any Kafka consumer group.

Messages in Kafka are serialized using a chosen format. When connectors read from or write to Kafka, they must perform deserialization or serialization as needed. To facilitate these operations, connectors may require access to the Schema Registry.

Kafka Connect also creates two additional internal topics within Kafka. The first, connect-statuses (the default name), stores status information for running connectors. The second, connect-offsets (default name), which we will explore in section 7.3.5, is used to track offsets for source connectors.

Importantly, Kafka Connect operates within the context of a single Kafka cluster, meaning all connectors and their configurations are tied to that specific cluster. This ensures that the internal topics and connector management are centralized within the same Kafka environment.

### 7.3.2 Internal Kafka Connect architecture

Kafka Connect servers form a cluster to enable scaling and fault tolerance. Each process that runs connectors within the Kafka Connect framework is known as a Kafka Connect worker. Workers manage the entire lifecycle of connectors, handling tasks like starting, stopping, and monitoring connector tasks. This makes workers essential to Kafka Connect’s overall operation. When running in containers, each container typically hosts one Kafka Connect worker. Figure 7.16 illustrates its internal architecture.

##### Figure 7.16 Connector tasks are distributed among workers, with each worker receiving its assigned portion.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image016.png)

Within each worker, the logic for copying data to or from Kafka is divided into _tasks_. Tasks run in parallel, and the connector configuration defines the maximum number of tasks allowed to run. Tasks may be configured differently; for example, each task might read data from a different database table. Kafka Connect distributes task configurations across workers, which then instantiate and execute tasks. Each task runs in its own thread to enable efficient parallel processing.

Kafka Connect workers in a cluster share the same group.id, even for source connectors, which act as producers rather than consumers. This shared group.id enables all workers to participate in group coordination. Workers send periodic heartbeats to the group coordinator, signaling that they are active. If a worker fails, it stops sending heartbeats, and the group coordinator detects this failure, redistributing its tasks to the remaining active workers.

It’s worth noting that while this shared group.id applies at the Connect worker level, individual connectors—particularly source connectors that internally use a Kafka consumer—may define and use their own group.id as part of their consumer configuration. This group ID governs the behavior of the embedded consumer and is distinct from the Connect worker group coordination.

In some cases, a task may also fail. For instance, if a connector sends data from Kafka to a database and the data violates the database’s integrity constraints, an exception is thrown, and the task enters a FAILED state. In this scenario, the task will not restart automatically. Therefore, it is essential to monitor task states closely to detect failures, resolve the root cause, and manually restart the task.

### 7.3.3 Converters

Different systems store data in their internal formats, and connectors need to understand them to be able to read or write the data. Source connectors read data from external systems and convert them to ConnectRecord, an internal model of Kafka Connect.

-   The class hierarchy is shown in figure 7.17.

##### Figure 7.17 ConnectRecord is the parent class for the SourceRecord and SinkRecord classes.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image017.png)

When Kafka Connect is used, we have an external system on one end and Kafka on the other. In the external system, data is stored in its native format, and in Kafka, data is stored in bytes, which can be produced by different serializers. Inside the Kafka Connect framework, the data is represented in the internal format of the Kafka framework. So, inside Kafka connector we have three different data formats:

-   Data in the external system, such as records in the database
-   Data inside Kafka Connect, represented by ConnectRecord objects
-   Data stored in Kafka, commonly using the Avro, JSON or Protobuf formats

Converters are responsible for converting data between ConnectRecords and Kafka. Each connector must specify a converter, depending on how data is stored in Kafka. The flow for source connectors is shown in figure 7.18.

##### Figure 7.18 How converters are used in the source connector flow

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image018.png)

Source connectors understand the format of the external system. They read data from those systems and convert it into ConnectRecord. Then the data is passed to the converter, which serializes it for storing in Kafka. Converters are specific to the format. In the example in figure 7.18, AvroConverter is used, which serializes data into Avro.

On the other hand, sink connectors must receive data from Kafka, and again this is the responsibility of the converter. The flow for sink connectors is shown in figure 7.19. Converters convert data from Kafka messages into ConnectRecord, and then sink connectors convert them into the format that the external system understands.

##### Figure 7.19 How converters are used in the sink connector flow

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image019.png)

As you can see, converters act as serializers and deserializers translating data between Kafka and ConnectRecords. As they operate only with Kafka and Kafka Connect’s format, they do not depend on the external systems and can be reused in different connectors.

### 7.3.4 Single message transformations

In many integration scenarios, the goal isn’t simply to transfer data between systems one-to-one but to perform various transformations. Common transformations include casting data types, filtering messages, and renaming attributes. Kafka Connect provides several built-in transformations ([https://docs.confluent.io/platform/current/connect/transforms/overview.html](https://docs.confluent.io/platform/current/connect/transforms/overview.html)), but it also allows developers to implement custom transformations in Java by implementing the org.apache.kafka.connect.transforms.Transformation interface.

Figure 7.20 illustrates how transformations are integrated into the Kafka Connect framework. These transformations are always stateless, processing one message at a time: a single message is received as input, and at most one modified message is produced as output. Since transformations cannot operate on multiple messages simultaneously, they are often referred to as Single Message Transformations (SMTs). The input and output of these transformations use the internal ConnectRecord format, making them reusable across different connectors. The transformation chain is static and defined in the connector’s configuration, meaning that any modifications require redeploying the connector.

##### Figure 7.20 Transformations are stateless and can be chained together.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image020.png)

For more complex data transformations, a streaming framework like Kafka Streams or Apache Flink may be more appropriate. We’ll explore complex data transformations further in chapter 8.

### 7.3.5 Source connectors

Source connectors pull data from external systems and load it into Kafka topics. They function as clients to these external systems, actively polling for data at regular intervals. For example, a connector may be configured to request data every five seconds.

Source connectors face two main challenges: how to load data in parallel, and how to determine whether data is new (i.e., how to identify data that has not yet been processed by the connector). Both challenges are specific to the external systems in use and typically cannot be solved in a generic way.

In Kafka Connect, the unit of parallelism is the task, with each task being assigned a different portion of work. Tasks are created when the connector starts, and the maximum number of tasks is defined in the connector’s configuration. A key responsibility of a source connector is to devise a strategy for parallel data loading. For example, when loading data from CSV files, each file can be assigned to a different task. When loading data from a database, different tables can be assigned to separate tasks, as illustrated in figure 7.21.

##### Figure 7.21 Database tables from the source database are assigned to the tasks in the source connector.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image021.png)

Another challenge is tracking which data has already been loaded. Some systems have natural data ordering, while others do not. In cases where data updates occur in a database table, the connector may not inherently know which data was modified. To handle this, the connector needs a type of _offset_ in the source system. For example, a database table might include an updated\_time column that records the timestamp of the latest data update. This timestamp then serves as an offset for the source connector, which stores it in Kafka’s connect-offsets topic. Each time the source connector loads data, it retrieves records with timestamps newer than the last stored offset. Figure 7.22 shows an example of this approach.

##### Figure 7.22 Update\_time is used as an offset for BalanceConnector. Each time, the connector loads data that was updated after the stored offset.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image022.png)

By design, source connectors retrieve data within each task, polling the external system at regular intervals. However, because connectors are Java-based applications, they also have the flexibility to spawn additional threads if more complex data exchange or asynchronous handling with the external system is required. For instance, a connector might launch a background thread to prefetch data, ensuring it is ready to be returned when the poll method is called.

### 7.3.6 Sink connectors

Unlike source connectors, sink connectors read data from Kafka, allowing them to utilize Kafka’s native partitioning and offset tracking. Acting as Kafka consumers, sink connectors can assign different partitions to separate tasks, and Kafka offsets help determine which data has already been sent to the target system. Sink connectors are illustrated in figure 7.23.

##### Figure 7.23 Sink connectors consume from Kafka with partitions balanced across tasks. Because tasks share a consumer group, each partition is assigned to only one task. Topic 2 has two partitions, so only two tasks consume it—task 3 gets no topic 2 assignment.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image023.png)

Sink connectors can be configured to handle errors that occur when they cannot push data to an external system. For instance, if a sink connector reads data from Kafka and attempts to insert it into a database, an error might occur if the data violates database integrity constraints. This is considered a non-retriable error, meaning retrying the insertion won’t resolve the issue, since the data itself must be corrected. Sink connectors have standard configuration options to manage such cases:

-   _Fail the connector_—In this case, an exception is thrown, and the connector stops running. This failure state requires monitoring to address the issue.
-   _Send messages to a dead-letter topic (DLT)_—The connector routes problematic messages to a special topic, known as a dead-letter topic, and continues processing other data. The DLT should be monitored, and alerts should be set up for when messages are sent to it.

The typical pattern for sink connectors is to consume data from Kafka and push it to external systems. However, since sink connectors are Java-based applications, they can perform additional background tasks as needed.

### 7.3.7 Changes in the incoming data structure

Connectors are designed to be dynamic, adapting to changes in the structure of incoming data. For instance, when loading database tables into Kafka, a source connector may load all tables from a specified schema. If new tables are added, the source connector should detect these changes and reconfigure tasks to incorporate the new tables. Source connectors also specify how to handle schema evolution. When the input data schema changes (for example, with the addition of a new column in a database table), the auto.register.schemas property controls whether the serializer should automatically register the new schema, ensuring that compatibility requirements are maintained.

If changes cannot be handled dynamically, such as when a connector tries to register an incompatible schema, the connector will fail. These errors are challenging to resolve, as they require stopping the connector, correcting the issue, and potentially resetting source connector offsets.

For sink connectors, tasks must be prepared to handle changes in partitions and input topics. Adding partitions to a topic triggers a consumer rebalance, reassigning partitions to tasks. Adding new topics may require the creation of corresponding resources in the target system, such as additional database tables.

### 7.3.8 Integrating Kafka and databases

Integrating Kafka with relational databases is one of the most common tasks in enterprise data pipelines. Let’s explore how this can be achieved, along with the available options and potential challenges.

One solution is to use JDBC source and sink connectors, which are available under a community license. As their name suggests, these connectors access databases through the JDBC protocol and support various database systems, as long as they are paired with the appropriate JDBC driver.

#### JDBC source connector

The JDBC source connector loads data from a database into Kafka. It can operate with multiple tasks, assigning each table to a different task, and dynamically adapting to database schema changes (such as added or removed tables). Each table is mapped to a corresponding Kafka topic.

The source connector polls tables to capture updated data, with several options for tracking new records:

-   _Bulk_—Periodically loads the entire table. This approach may produce duplicate entries, which consumers must handle. Since it loads the full table, it is unsuitable for large tables.
-   _Incrementing_—Uses a column with incrementing values (typically a database sequence, which is used as a primary key) to track new records. This mode only detects inserts, making it ideal for immutable data but unable to track updates.
-   _Timestamp_—Uses a timestamp column that updates whenever a row is modified, tracking rows by the last modified time.
-   _Incrementing + Timestamp_—Combines incrementing and timestamp columns, identifying rows by both values for more accurate tracking.
-   _Query_—Selects data using a custom query instead of polling the entire table. This mode can be used with incrementing or timestamp tracking, or the query itself must manage offset tracking. Additionally, query mode allows developers to join multiple tables directly in the query, enabling more complex data extraction scenarios such as enriching records or consolidating data from related tables.

Most standard data types convert seamlessly to ConnectRecord, though custom or database-specific types may require extra handling. The JDBC source connector does not support hard deletes; if a row is deleted from the database, the connector cannot detect this. However, soft deletes are possible by using a special column that flags rows as deleted.

#### JDBC sink connector

The JDBC sink connector writes data from Kafka into a target database. It can also run multiple tasks, each assigned to a subset of Kafka partitions, allowing for parallel writes. Each Kafka topic maps to a corresponding table in the database.

There are two modes for writing data:

-   _Insert_—The connector generates INSERT statements.
-   _Upsert_—Performs idempotent writes by either inserting a new row if no matching primary key exists or updating the row if it does.

When the sink connector encounters a tombstone message (a message with a null value), it deletes the record in the database that matches the specified key.

JDBC connectors can be somewhat restrictive, as they require specific table structures, including designated columns for tracking updates, and they do not support hard deletes.

#### Integration alternatives

An alternative for integrating Kafka with databases is to use connectors that directly access the database’s transaction logs, like Debezium ([https://debezium.io](https://debezium.io/)). Instead of issuing SELECT queries, these connectors capture data directly from the transaction logs, bypassing the need for schema modifications and allowing them to capture all types of database changes, including inserts, updates, and deletes.

A significant drawback, however, is that the transaction log must be exposed to the connector, which can raise security concerns.

### 7.3.9 Creating a connector for the Customer 360 ODS

The idea of integrating data purely through configuration is appealing—it promises a quick setup with minimal development time and is worth experimenting with. For architects, it might be a nostalgic reminder of the days when data transfers were configured with a few well-placed scripts. From the data engineering perspective, however, there may be some concerns about the security of this approach. Working at such a low level may feel like it’s exposing the entire database model directly to the integration layer, essentially bypassing data contracts. Isolating the integration work by setting up a separate cluster dedicated to Kafka Connect is a safer option.

Instead of the usual TransactionService, the architects of an ODS project like this may opt to use a source connector to load data directly from the database table into the TRANSACTIONS topic in Kafka. To do this, you’d install Kafka Connect, deploy the JDBC source plugin, and prepare the configuration, as follows.

##### Listing 7.1 Configuration of the JDBC source connector

```
{ "name": "jdbc-source-transactions", 
  "config": 
    {"connector.class": "JdbcSource",  #A
     "tasks.max": 1,  #B
     "topic.prefix": "connect-",  #C
     "connection.url": "jdbc:postgresql://postgres:5432/trans_db",  #D
     "mode": "incrementing",  #E
     "incrementing.column.name":"ID",  #F
     "value.converter": "io.confluent.connect.avro.AvroConverter",  #G
     "value.converter.schema.registry.url":"http://schema-registry:8081",  #H
     "table.whitelist" : "public.transactions",  #I
     "connection.user": "demo", 
     "connection.password" : "demo"
    }
}    
```

Now you can deploy this configuration to the Kafka Connect server via the REST API, and the connector will immediately begin transferring data from the database into Kafka.

Since transactions are immutable, the design team on this ODS uses an incrementing column, ID, to track and load new records efficiently.

### 7.3.10 Common Kafka Connect problems

Implementing data integrations with Kafka Connect significantly simplifies development; however, it also comes with certain drawbacks.

#### Data type mismatches

Types in external systems often do not map directly to the types supported by Kafka serializers. While connectors attempt to find the best matches, certain types—such as enumerations, geospatial data, and complex structures like arrays or structs—require special handling and cannot be directly mapped. Even Boolean values may vary in representation, appearing as 0/1, Y/N, or true/false. For types without a direct representation in Kafka’s supported formats, implementing custom serializers or message transformations is necessary.

#### Defining offsets in the source system

Using databases as an example, tracking which data has already been loaded into Kafka can be challenging. Some systems only expose the current state without providing mechanisms to track updated data. In such cases, a full data dump may be necessary, which is resource-intensive and can lead to duplicate records in Kafka.

#### Parallelizing loading of data

Loading data in parallel is challenging in systems that lack natural partitioning. If a source system consists of a single large CSV file, splitting it into chunks for parallel loading may require manual processing, as the file itself lacks partitions. In these cases, connectors that cannot parallelize data loading are limited to a single task, which significantly impacts performance by restricting throughput.

#### Imbalance in task distribution

Kafka Connect aims to distribute tasks evenly across workers, but achieving a balanced load is often challenging. For example, when loading database tables, the connector may assign an equal number of tables to each task. However, tables often vary significantly in the number of records or update frequency, leading to imbalanced workloads where some workers are underutilized while others are overloaded. This uneven distribution is a common issue, especially in systems where data volumes and update rates differ widely across tasks.

#### Calculating memory requirements

Estimating memory requirements for Kafka Connect is tricky, particularly in environments where multiple connectors are running simultaneously, each with unique workloads. Memory usage varies widely depending on factors such as data volume, connector type, task parallelism, and transformation complexity. For example, source connectors handling large batch loads or performing complex transformations may consume significantly more memory than lightweight connectors with minimal processing. Additionally, memory usage can fluctuate based on peak data flows and buffering needs, making it difficult to set precise resource allocations. As a result, memory planning for Kafka Connect often requires careful monitoring, tuning, and scaling to prevent bottlenecks and ensure stable performance across all connectors.

#### Schema evolution and poison pill messages

A serious issue in Kafka Connect is the presence of poison pill messages—messages that are corrupted or incompatible with one of the systems involved. For instance, if the schema in the source system changes (such as with the addition of a new column to a table), it may no longer align with the Avro schema expected in Kafka. As a result, attempts to write this message to the topic fail, causing the connector to stop, since source connectors lack dead-letter topic (DLT) support. Such errors require careful monitoring to ensure timely resolution.

For sink connectors, schema incompatibility with the target system is a common cause of failures. In this case, incompatible messages can be redirected to a DLT without interrupting the connector’s operation. However, it’s crucial to monitor and alert on these events to avoid data loss or processing delays.

This issue is compounded by the fact that data replication in Kafka Connect operates at a granular level, replicating the data field by field rather than as an abstract service or high-level entity. This fine-grained replication requires precise schema alignment across all participating systems. As a result, coordinating schema changes becomes a significant challenge, as each system must be aware of and prepared for updates as they occur.

#### Management of DLT

When a sink connector encounters non-retriable errors, messages may be sent to a DLT. However, Kafka does not provide built-in management for DLTs, so all processes involving this topic require custom implementations. It falls to the operations team to establish monitoring for the DLT and to determine how to handle messages that accumulate there.

Non-retriable errors can stem from various issues. For instance, a message may have an incompatible schema and be redirected to the DLT. Once the target system’s schema is updated—often by the team responsible for that system—the messages resume processing successfully. However, messages sent to the DLT are lost from the main flow, creating a partial data loss. This makes it challenging to restore the original event order and reapply messages correctly. In such cases, teams frequently implement reconciliation processes, and in some cases, they may need to reload the entire dataset from scratch.

## 7.4 Ensuring delivery guarantees

One of the most serious concerns in asynchronous message transfer is the risk of undetected data loss. If there’s a suspicion that data has been lost, pinpointing the issue can be challenging. Several scenarios could lead to data loss:

-   The message was never sent by the producer system.
-   The message was not written to Kafka brokers, and the lack of acknowledgment was not properly handled.
-   The message was written to Kafka and acknowledged by the leader, but the leader failed before replicating the message to followers.
-   The message was deleted due to the retention policy before it was consumed.
-   The message was consumed, the offset was committed, but an exception occurred during processing that was not properly handled.

The common solution is to trace messages throughout the entire pipeline, logging each step. However, tracing a high volume of messages can be resource-intensive.

Another challenge is the potential for duplicates in Kafka. If a batch of messages is published and acknowledged, but the acknowledgment is lost, the producer may resend the batch, leading to duplicate messages. Duplicates can also occur on the consumer side if the consumer fails before committing offsets of processed data, causing previously processed data to be read again.

Kafka provides certain guarantees to prevent data loss and duplication when specific criteria are met. Let’s examine these guarantees in detail.

### 7.4.1 Producer idempotence

Producer idempotency in Kafka prevents duplicate and out-of-order messages by ensuring that each message is written only once to a partition, even if retries occur. Idempotency addresses several potential issues, such as these:

-   _Duplicate messages_—If a batch of messages is sent and acknowledged by the broker, but the acknowledgment is lost, the producer might resend the batch. Without idempotency, this would lead to duplicate messages.
-   _Out-of-order messages_—If a producer sends multiple batches to the same partition, batch order can become disrupted if acknowledgments fail. For example,

1.  The producer sends batch 1, which fails to acknowledge.
2.  The producer then sends batch 2, which is successfully acknowledged.
3.  If batch 1 is resent and written after batch 2, the messages are stored out of order in the partition.

In earlier Kafka versions, setting max.in.flight.requests.per.connection (number of unacknowledged requests) to 1 helped avoid out-of-order messages by ensuring only one batch could be in flight per partition. However, recent Kafka versions (starting from 0.11.0) have made idempotency simpler and more robust with the default setting of enable.idempotence=true. This setting causes Kafka to assign each producer a unique identifier and each batch a monotonically increasing sequence number for each partition. When a broker receives a batch, it expects a specific sequence number:

-   If the sequence number matches, the batch is written, and the broker updates the expected sequence.
-   If a batch arrives with an out-of-sequence number, the broker detects this and throws a retriable exception, allowing the producer to retry sending the batches in the correct order.

Kafka’s idempotency requires a few additional configuration settings:

-   max.in.flight.requests.per.connection—Set to 5 or fewer to avoid concurrency issues.
-   retries—Set to a positive number, allowing retries on transient errors.
-   acks—Set to all, ensuring data is acknowledged only after replication across all in-sync replicas.

These configurations ensure that Kafka can maintain a strict order of events in each partition and prevent duplicate messages, providing the highest level of message delivery guarantee. The sequence numbers and producer identifiers operate internally and are not accessible via the Kafka API.

### 7.4.2 Understanding Kafka transactions

One of the most common questions is whether Kafka supports transactions. Before answering, let’s clarify what we mean by a transaction. In databases, _transactions_ are processes that adhere to four key properties, known as ACID:

-   _Atomicity_—Ensures that all operations within a transaction are completed successfully, or none are. If any part of the transaction fails, the entire transaction is rolled back, preventing partial updates.
-   _Consistency_—Ensures that transactions take the system from one valid state to another, maintaining data integrity based on defined rules or constraints.
-   _Isolation_—Guarantees that transactions are executed independently, so intermediate states are not visible to other concurrent transactions.
-   _Durability_—Ensures that once a transaction is committed, its changes are permanent, even in the event of a system failure.

In Kafka, transactions are slightly different. Kafka’s transactions are designed specifically for stream processing applications in a read-process-write pipeline, where an application reads events from a topic, processes them, and produces results to one or more output topics. Kafka transactions guarantee exactly-once semantics in this pipeline, meaning each input event is processed exactly once, with outputs produced only once, even in the event of retries or failures.

Figure 7.24 illustrates this with an example. Suppose we have an OrderService that reads data from the ORDERS topic, processes it, and produces events to three output topics: SHIPMENTS, BILLING, and INVENTORY. Enabling exactly-once semantics ensures that for each incoming Order event, the service generates precisely one corresponding Shipment event, one Billing event, and one Inventory event, even in cases of service failure. In this setup, exactly-once processing is essential. Duplicating or skipping any Order event would lead to incorrect downstream data and significant discrepancies.

##### Figure 7.24 OrderService reads, processes, and writes each event atomically.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image024.png)

Here’s how exactly-once semantics ensures consistency in this scenario:

-   _Read_—The service reads order data from the ORDERS topic. Each order event includes details such as product information, quantity, payment amount, and shipping details.
-   _Process_—The service processes each order, generating three different events based on the incoming data.
-   _Write_—The service then writes three output events:

-   Shipment—Contains information for shipping the order.
-   Billing—Contains information for invoicing the order.
-   Inventory—Contains information for updating inventory levels.

With exactly-once semantics enabled, Kafka guarantees that all three events (Shipment, Billing, and Inventory) appear exactly once per each incoming order, even if retries occur. This transactional setup ensures atomicity for the entire read-process-write cycle, meaning the operation is all or nothing: either all events are successfully processed and written, or none are. If the service fails before commit, none of the outcoming events are visible and the input will be reprocessed.

Kafka ensures this behavior by enabling atomic writes across multiple partitions, as shown in figure 7.25. The process requires atomic confirmation of both reading the input and producing the output. Confirming that data has been read is equivalent to performing a write to the \_\_consumer\_offsets topic, which marks the offset at which processing occurred. To achieve atomicity, a transaction must perform atomic writes to the following:

-   The \_\_consumer\_offsets topic to confirm the read and processing of the Order event
-   The SHIPMENTS topic
-   The BILLING topic
-   The INVENTORY topic

##### Figure 7.25 A transaction involves writing atomically to all four topics.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image025.png)

All these operations must succeed together or all fail together. Kafka transactions allow these operations to be executed as a single atomic unit, ensuring exactly-once processing throughout the pipeline.

While transactions in Kafka were originally designed for stream-processing frameworks, where they are managed automatically, this feature can also be utilized independently. Any producer application can leverage Kafka’s transaction API to send multiple messages atomically. Using methods such as beginTransaction, commitTransaction, and abortTransaction, producers can group multiple writes into a single atomic operation, as illustrated in figure 7.26. This flexibility makes Kafka transactions a powerful tool for ensuring consistent state changes across multiple topics, even outside the context of stream processing.

##### Figure 7.26 Creating a transaction using ClientAPI. Both send operations are part of a single transaction, ensuring that they either both succeed or both fail atomically.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image026.png)

Kafka’s transaction mechanism seamlessly integrates the process of writing and reading transactional messages. While the producer API enables atomic writes across multiple partitions, the way these messages are consumed depends on the isolation level configured for the consumer. The isolation.level consumer property has two possible values:

-   read\_committed (default)—Returns only committed transactional messages, filtering out any aborted messages.
-   read\_uncommitted—Returns all messages, including both committed and aborted transactional messages.

Nontransactional messages are returned in both modes. Figure 7.27 demonstrates the difference between these isolation levels.

##### Figure 7.27 Isolation levels determine whether consumers can read uncommitted messages. This figure shows two consumers with different isolation settings.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image027.png)

Open transactions that are neither committed nor aborted can cause performance issues. Transactional consumers encountering uncommitted messages cannot proceed and must wait until the transaction is completed. If the producer does not commit or abort the transaction, it will eventually be aborted due to a timeout.

### 7.4.3 Transactional outbox pattern

A common question is whether Kafka can participate in distributed transactions that involve multiple systems. Distributed transactions are transactions that span multiple independent systems or databases, ensuring that all operations across these systems are either completed successfully or none are, maintaining consistency across the distributed environment. The most widely used protocol for this is the two-phase commit (2PC) protocol, which coordinates transactions in two main steps:

-   _Prepare_—The transaction manager sends a “prepare” request to each participant, prompting them to complete their part of the transaction. Each participant responds with either a “ready to commit” or a “failure” message.
-   _Commit_—If all participants are ready, the transaction manager sends a “commit” request to finalize the transaction. If any participant fails, the transaction manager issues a request to abort the transaction for all participants, ensuring consistency by rolling back.

Currently, Kafka does not support the 2PC protocol. However, support for distributed transactions is proposed in KIP-939 ([https://cwiki.apache.org/confluence/display/KAFKA/KIP-939%3A+Support+Participation+in+2PC](https://cwiki.apache.org/confluence/display/KAFKA/KIP-939%3A+Support+Participation+in+2PC)), which aims to introduce 2PC capabilities in Kafka.

When Kafka is involved in a distributed process, we often encounter the dual-write problem. For example, consider a scenario where a user updates their profile via a web interface. When the ProfileService receives this new data, it needs to perform two actions:

-   Save the updated profile data to the internal transactional database.
-   Send an event to Kafka to notify other systems of the profile change.

Since Kafka does not support the 2PC protocol, these two actions cannot be performed as part of a single distributed transaction. This raises the question: In what order should these steps be executed?

If the database update succeeds but sending the message to Kafka fails, the data becomes inconsistent across systems. Conversely, if the event is sent first, the database update could fail, leading to a similar inconsistency. How can this issue be handled?

One solution is the _transactional outbox_ pattern, shown in figure 7.28. In this approach, a special outbox table (shown as ProfileEvents in the figure) is created to hold event data. Whenever a profile is updated, an event record is inserted into the outbox table within the same database transaction. This ensures that either both steps succeed, or both are rolled back together. A separate process then monitors the outbox table and sends new events to Kafka, retrying until successful. After sending, it either deletes the successfully published events from the table or tracks the last sent event ID.

##### Figure 7.28 When the transactional outbox pattern is used, a separate process (step 3 in the figure) publishes events to Kafka.

![](https://drek4537l1klr.cloudfront.net/gorshkova/v-8/Figures/ch07__image028.png)

In essence, this pattern shifts the dual-write problem to the outbox cleanup process, which lacks an inherent guarantee that event deletion and publishing to Kafka will be atomic. If deletion and publishing are not perfectly synchronized, this could result in duplicate messages in Kafka if messages are resent, or orphaned records in the outbox if cleanup fails.

In cases where cleanup or sending fails, duplicate events may be published, which can be managed by designing consumers to handle idempotent processing. Additionally, many teams implement reconciliation processes or, if necessary, a complete data reload to ensure consistency.

The transactional outbox is often implemented as a separate microservice process. However, with Kafka, it may be beneficial to implement this process as a Kafka connector using Kafka Connect, which can monitor the outbox table and handle event publishing efficiently.

## 7.5 Online resources

-   “What is a Service Mesh?” [https://konghq.com/blog/learning-center/what-is-a-service-mesh](https://konghq.com/blog/learning-center/what-is-a-service-mesh)
-   A tutorial-style overview explaining the purpose and core concepts of a service mesh.
-   “Kafka Mesh filter”: [www.envoyproxy.io/docs/envoy/latest/configuration/listeners/network\_filters/kafka\_mesh\_filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/listeners/network_filters/kafka_mesh_filter)
-   Documentation describing Envoy’s Kafka mesh filter and how to configure it.
-   “Synchronous Kafka: Using Spring Request-Reply”: [https://dzone.com/articles/synchronous-kafka-using-spring-request-reply-1](https://dzone.com/articles/synchronous-kafka-using-spring-request-reply-1)
-   A step-by-step guide to building a synchronous request-reply workflow in Kafka with Spring’s request-reply template.
-   “Discover Kafka: connectors and more”: [www.confluent.io/hub](https://www.confluent.io/hub/)
-   A marketplace of officially supported and community-built Kafka connectors available for download.
-   “Kafka Connect Architecture”: [https://docs.confluent.io/platform/current/connect/design.html](https://docs.confluent.io/platform/current/connect/design.html)
-   Documentation explaining the design principles and architecture of Kafka Connect.
-   “Kafka Connect Deep Dive—Converters and Serialization Explained”: [www.confluent.io/blog/kafka-connect-deep-dive-converters-serialization-explained](https://www.confluent.io/blog/kafka-connect-deep-dive-converters-serialization-explained/)
-   A detailed explanation of how converters and serialization work inside Kafka Connect.
-   “Transactions in Apache Kafka”: [www.confluent.io/blog/transactions-apache-kafka](https://www.confluent.io/blog/transactions-apache-kafka/)
-   A guide to Kafka’s transactional model, including exactly-once semantics and producer guarantees.
-   “KIP-939: Support Participation in 2PC”: [https://cwiki.apache.org/confluence/display/KAFKA/KIP-939%3A+Support+Participation+in+2PC](https://cwiki.apache.org/confluence/display/KAFKA/KIP-939%3A+Support+Participation+in+2PC)
-   A Kafka Improvement Proposal outlining support for integrating Kafka producers into two-phase commit (2PC) transactions.

## 7.6 Summary

-   Kafka is best suited for publish-subscribe, event-driven architectures, rather than traditional request-response models.
-   Kafka follows a “smart endpoints, dumb pipes” model, where message processing logic is handled at endpoints, leaving Kafka as a pure transport layer.
-   The service mesh does not fully align with Kafka’s client libraries; however, experimental solutions, such as Kafka mesh filters, are emerging to bridge this gap.
-   Kafka supports CQRS well, separating read and write operations across services and enabling event-driven updates to query models.
-   Kafka supports event sourcing, but snapshotting is often necessary to optimize data restoration performance for consumers.
-   Kafka can support data mesh architectures by enabling decentralized data ownership, data-as-a-product principles, federated governance, and self-serve data platforms.
-   Kafka Connect simplifies data integration with source and sink connectors, though managing schema evolution and handling errors (e.g., poison pill messages) can be challenging.
-   Kafka’s exactly-once semantics ensures that data is processed exactly once per transaction in a read, process, write pipeline, maintaining atomicity across multipartition writes.
-   Kafka does not currently support two-phase commit (2PC) but may offer this in the future (e.g., KIP-939); the transactional outbox pattern is often used as a workaround.