-   The basics of Command Query Responsibility Segregation
-   CQRS in reactive systems
-   Commands and queries
-   CQRS and Event Sourcing combined

As you’ve seen in the previous chapters, the reactive paradigm is a powerful way to think about distributed computing. In this chapter, we build on that foundation by exploring two design patterns introduced in [chapter 1](https://livebook.manning.com/book/reactive-application-development/chapter-1/ch01): Command Query Responsibility Segregation (CQRS) and Event Sourcing (ES). We should note, however, that although these techniques work in concert and are a natural fit for reactive programming, they’re by no means the only way to design a reactive system.

To achieve our goal of exploring these two design patterns, we look at a common type of application: the database-driven application, which is the foundation of many, if not most, systems built today. We explore the challenges in building these types of applications in a distributed environment and show you how, through the reactive paradigm, you can use CQRS/ES to overcome these challenges.

First, we need to state what we mean by _database-driven application_. The term can mean many things to many people, but at its root is a simple meaning: a software application that takes in and persists data and provides a means to retrieve that data.

After discussing the driving factors toward CQRS/ES, we explain the concepts and alternatives to a typical monolithic, database-driven application. We look beyond the theoretical notion of reactive applications being message-driven and ground that concept in a reality that uses CQRS commands and Event Sourcing events as actual messages.

### 8.1. Driving factors toward CQRS/ES

The relational database management system (RDBMS), traditionally in the form of a Structured Query Language (SQL) database, has directly affected how applications are built. Frameworks such as Java Enterprise Edition (Java EE), Spring, and Visual Studio made SQL the foundational norm for system development as we entered the age of affordable and more agile computing. Before SQL, mainframe systems were the central computing areas of business, and users were limited to using dumb terminals, reports, and so on. SQL databases offered an accessible, easy-to-use storage model; programmers quickly became dependent on the capabilities they offered and even built systems around them. The growing number of desktop PCs led to a growing number of systems built against these databases, and the creation of systems free from rigid management information systems fiefdoms was great for computing overall. Unfortunately, traditional solutions have clear limitations, preventing application distribution and usually resulting in monolithic designs such as _ACID transactions_. We discuss ACID transactions, the pitfalls of relational databases, and CRUD in the following sections.

#### 8.1.1. ACID transactions

Typically, monolithic applications allow database transactions across domain boundaries. In this world, it’s perfectly possible for a transaction to guarantee that a customer was added to a customer table before an order was added to an order table; we were able to make these separate table writes all at once because all of the data resided in the same database. These transactions are typically referred to as _ACID transactions,_<sup>[<a href="https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fn1">1</a>]</sup> which have the following properties:

-   _Atomicity_ allows multiple actions to be performed as a single unit. Atomicity is the basis of transactions in which an update to a customer address can take place in the same unit of work as creating an order for that customer.
-   _Consistency_ means strong consistency, as all changes to data are seen by all processes at the same time and in the same order. This is also a characteristic of transactions and is so expensive that processes dependent on this type of consistency, such as traditional database-driven applications (usually, monolithic), can’t distribute.
-   _Isolation_ provides safety against other concurrent transactions. Isolation dictates how data in the act of change behaves when read by other processes. Lower isolation levels provide greater concurrent access to data but a greater chance of concurrent side effects on the data (reads on stale data). Higher isolation levels yield more purity on reads but are more expensive and come with the risk of processes blocking other processes, or, worse, deadlocks, when two or more writes block each other indefinitely.
-   _Durability_ means that transactions, once committed, survive power outages or hardware failures.

A reactive application is capable of being distributed. It’s impossible to have ACID transactions across distributed systems because the data is located across geographic, network, or hardware boundaries. There’s no such thing as an ACID transaction across these types of boundaries, so unfortunately, you need to cut the cord of traditional RDBMS dependence. Distributed systems can’t be ACID-compliant.

We say _traditional RDBMS_ because some relational databases, such as PostgreSQL, have evolved and overcome relational limitations, with features such as object relational storage and asynchronous replication allowing distribution.

#### 8.1.2. Traditional RDBMS lack of sharding

These RDBMS-based systems don’t shard. _Sharding_ is a way of spreading your data by using some shard key. You can use sharding to intelligently co-locate data and also to distribute that data. Sharding provides horizontal scaling across machine hardware and geographical boundaries; therefore, it allows elasticity and distribution of data. An example of sharding is a photo-sharing application that uses a user’s unique ID as the shard key. All the user’s photos are stored in the same area of the database. Without the ability to shard, a traditional RDBMS is reduced to throwing more hardware and storage at the problem for the same physical database, which is called _vertical scaling_, and you can scale only so far with interconnected hardware. _Horizontal scaling_ is scaling across geographical or machine boundaries. Sharding is a way of achieving distribution and paves the way for dividing systems in terms of CQRS.

#### 8.1.3. CRUD

As we discuss in [chapter 1](https://livebook.manning.com/book/reactive-application-development/chapter-1/ch01), Create, Read, Update, Delete (CRUD) is the in-place modification of a piece of data in a single location. With CRUD, all updates are destructive, losing all sense of the previous state of the data, including deletions, which are the most radical updates of all. With CRUD, valuable data is constantly lost. There’s no concept of state transition—the current state of any object is all you get. A completed order, for example, is just that. All notions of the new order, fulfilled order, in-process order, and so on are lost. No history exists of the behavior of any given order, making it impossible to trace how it got from point A to point Z. CRUD isn’t easily distributable, in that the domain is mutable. Any distributed entity could be changed anywhere at any time, and it’s difficult to know the single source of truth. With CRUD, you always have a single source of truth; all other references to the CRUD entity are made by copy or reference only.

Far and away the most common use of CRUD is in a relational database. In creating the database structure, follow widely accepted best practices so that you have no redundant data, and allow relationships through the use of primary and foreign keys. A foreign key, for example, is a customer table associated with a concrete, associated table of orders and joined by a foreign key field on customer in the orders table. The CRUD model can’t scale because of the interdependency of the data constructs, which are bound together. _If the only way you can distribute is to distribute the entire thing, you haven’t distributed anything._ The table structures follow the patterns of what you think your business objects will look like, usually through the use of hierarchical relationships. Then you attempt to layer on top an object-oriented domain structure to tie everything together. Although this approach is the foundation of many an application, the implied costs can be painful. Sometimes, CRUD is perfectly fine for simple applications. We’ll use a contacts system as an example of such a simple system. This application to maintain contacts has no additional views, just the attributes of the contact as stored in the database. Furthermore, the contact has no relationships to any other domain, or, ideally, few other domains, and is effectively stand-alone. You should understand that it’s easy to build this type of application, and if the simplicity of the system warrants it, using this type of application is perfectly fine. In most of the real work applications we’ve seen, however, CRUD isn’t enough or is incorrect. You need another solution and a way to do away with all this pain. For those systems, apply CQRS, usually in combination with Event Sourcing.

### 8.2. CQRS origins: command, queries, and two distinct paths

Although many people are only now familiarizing themselves with CQRS, it has been around since early 2008. It was crafted by Greg Young, an independent consultant and entrepreneur. As Young puts it, “CQRS is simply the creation of two objects where there was previously only one. The separation occurs based upon whether the methods are a Command or a Query. This definition is the same used by Meyer in Command and Query Separation: a command is any method that mutates state and a query is any method that returns a value.”

The single object you’re used to is divided in two (queries and commands), and the two objects have distinct paths. A real-life example is the creation of an order, which is done against an order command object or module. The viewing of open orders would be against an order query module.

As its name portrays, CQRS is about commands, queries, and their segregation. We talk about these concepts in detail throughout this chapter, but in this section, we focus on segregation.

Because CQRS combines so nicely with Event Sourcing, we usually refer to the systems that we build with this model as CQRS/ES.

Unfortunately, the word _segregation_ (the _S_ in _CQRS_) often has a negative connotation, as it does when applied to human relations. When used in the context of reactive systems, however, segregation is a focal point for resilience. The dictionary defines _segregation_ as “the action or state of setting someone or something apart from other people or things.” This setting apart empowers CQRS/ES as a reactive pattern. Segregation in essence isolates each side of a CQRS/ES system, providing fault tolerance to the system as a whole. If one side goes down, complete system failure doesn’t result, because one side is isolated from the other. This pattern is commonly referred to as _bulkheading._

Bulkheading originated in the shipping industry, as shown in [figure 8.1](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig01), in which bulkheads are watertight compartments designed to isolate punctures in the hull. [Figure 8.1](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig01) (drawn by a famous New York City artist) has several bulkheads. If a bulkhead is compromised, the ship can remain afloat.

##### Figure 8.1. The SS _Not Titanic_. Bulkheaded compartments ensure that damage of one or more bulkheads doesn’t take down the ship.

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig01_alt.jpg)

As the figure shows, if a bulkhead is compromised, water is prevented from flowing into another bulkhead, limiting the scope of failure. For the ship to sink, total failure, multiple bulkheads would have to fail.

The bulkheading pattern in CQRS can be demonstrated by a failure on the query (domain side) that precludes making any changes in an area of your domain. Because you’ve implemented the command side, all the data necessary to read the last known state of the domain is still available, and clients of that data are unaffected in terms of reads. We discuss bulkheading as it applies to CQRS later in this section.

Another aspect of segregation optimizes data writing versus reading. Rather than using a single pipeline to process writes (commands) and reads (queries), as in a monolithic CRUD application), CQRS implements two distinct paths: commands and queries, thereby achieving a measure of bulkheading.

This division shadows the Single Responsibility Principle (SRP), which states that every context (service, class, function, and so on) should have only one reason to change—in essence, a single responsibility. The single responsibility of the command side is to accept commands and mutate the domain within; the single responsibility of the query side is to provide views on various domains and processes to make client consumption of that query data as simple as possible. Through segregating and focusing on a single goal for each path, you can refine writes and reads independently. Also, you have bulkheading in that the command side functions independently of the query side, and any failures on one side don’t directly affect the responsiveness of the other side.

Many applications have significant imbalances between reads and writes. Systems such as high-volume trading or energy may have a staggering number of writes in terms of trades or energy readings but much less viewing of that data, or at least the data is aggregated before it’s read. The query side typically requires complex business logic in the form of _aggregate projections_, which are views on the domain(s) that usually don’t look quite like the domain itself. An example is a view that includes customer information in detail, as well as order history and a list of sales contacts. The command side wants to persist. A single model encapsulating both tasks does neither well.

In [figure 8.2](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig02), you see four CQRS command modules representing the sales, customer, inventory, and billing domains. Each domain is purpose-built to concentrate only on its narrow area of focus, with little concern about the other modules or how the data will be queried. The domain concerns are neatly modeled in each of the command modules; the Order to Cash query module handles all the heavy lifting required for presentation, which includes joining and pivoting the data to fit the client’s needs. The Order to Cash query module is a first-class citizen in CQRS and exists solely to accumulate and return data across all the command modules.

##### Figure 8.2. Two distinct paths: purpose-built domain and query modules focusing on a single duty

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig02_alt.jpg)

In [figure 8.2](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig02), you see that reads (queries) and writes (commands) follow their own discrete paths, independent of each other in real time, meaning that there’s no connection between read and write data during reads and writes. Keep in mind that an asynchronous relationship exists between the command and query sides and the constant syncing up over time from the command to the query sides. The query data is always waiting to serve the clients in the last known state, offering the highest performance and simplicity in terms of reads. The query store exists for the sole purpose of serving the clients; it may be built atop multiple domains or data feeds. Clients may use the read data to construct commands on the domains (command sides). Those commands are always sent to a single command side and may result in a change to the query side, but in an eventually consistent, asynchronous manner.

[Figure 8.3](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig03) shows how bulkheading applies to a CQRS system.

##### Figure 8.3. The natural bulkheading of CQRS. Two command modules are unavailable, but the rest of the system remains functional.

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig03_alt.jpg)

In [figure 8.3](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig03), you see that the sales and customer command systems have gone down or become unavailable. The natural bulkheading that CQRS provides allows users full access to the data necessary to populate displays and issue commands, but those commands will fail against the systems that are down. You could queue up commands somewhere so that they don’t fail, but this attempt is of little value, because the commands have no effect until the failed systems become available once again. A better design would be something like a redundant system to mitigate these situations. The main point is that the possibility of failure always exists; the best thing you can do is contain that failure in the best way possible.

### 8.3. The C in CQRS

The _command_ side, sometimes called the write side, is about more than commands. It represents the domain only. Theoretically, the command side is never meant to be read directly.<sup>[<a href="https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fn2">2</a>]</sup> The command side is tuned for high-throughput writes and may have specific database and/or middleware selections for this purpose, differing from the query sides. In this chapter, we show CQRS with Event Sourcing, which is the most valuable pattern in our experience, but there’s nothing to stop you from using CQRS alone when read and write separation is a requirement.

<sup><a id="ch08fn2">2</a></sup>Recent technologies such as Akka persistence allow in-memory domain state to provide simple and performant reads of the domain, but the lion’s share of reads is left to the query side.

#### 8.3.1. What is a command?

Before we define a command, review our definition of _message_ from [chapter 1](https://livebook.manning.com/book/reactive-application-development/chapter-1/ch01): an immutable data structure used to communicate across service boundaries. One of the principal attributes of a reactive system is that it’s message-driven. One type of message that allows message-driveness is a command. Commands are sent from clients or services to various other CQRS services.

A _command_ is a request to do something, and that something usually is a request to change state. A command is imperative. Although it’s authoritative in nature, it’s a desire to have a system take an action, and as such, it may be rejected due to validation errors. Commands follow verb–noun format, in which the verb is the action requested and the noun is the recipient of that action. Following are two typical commands:

```
CreateOrder(...)
AddOrderLine(...)
```

#### 8.3.2. Rejection

Rejection is an important concept in a CQRS system, but it can be confusing in dealing with the notion of a command. Because commands are authoritative in nature, acceptance is often assumed, because a command is an imperative delivered by the authority. As we mentioned earlier in this chapter, in the context of CQRS, a command is more a request than an order, so the recipient is at liberty to reject it. Rejection generally is the result of some form of business-rule violation or attribute-validation failure. An example is the rejection of a CreateOrder command. This request might be rejected because the sender doesn’t have the proper credentials to create orders or has an overdrawn credit line, for example.

As we discuss in [chapter 4](https://livebook.manning.com/book/reactive-application-development/chapter-4/ch04), the command handler on the command side is also an anticorruption layer on its associated domain. Another interesting aspect of a command is its structure, in that it may contain multiple attributes requiring validation. This structure presents an intriguing question: When rejection occurs, is one error at a time or a list of errors reported back to the sender? We explore the answer to this in [section 8.3.7](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08lev2sec10). Command validation comes with a cost in terms of performance, so it isn’t used sometimes, as in cases of high-volume trading.

#### 8.3.3. Atomicity

There seems to be overwhelming buy-in on the microservice paradigm, in large part because of the popularity of the Reactive Manifesto. The microservice paradigm dictates building many services, each doing a few focused things, which implies atomicity. _Atomicity_ means that each service solves a particular problem without knowledge of or concern about the internal workings of other services around it. This philosophy falls in comfortably with the Reactive Manifesto, so much so that it could easily be the fifth pillar.

Atomicity allows solutions to smaller problems. The smaller the problem and the larger the isolation of that problem, the easier it is to solve. Try to avoid cross-cutting concerns in your service code. If an external concern or edge case tries to work into your service, find a way to handle such cases elegantly and in a nonspecific way. Consider a service that applies discounts for books. One business case applies a greater discount if the book is bought in a store rather than online. Rather than add the ability to track whether any particular book was bought in a store, you could support this functionality in a more abstract manner, such as by adding a discount attribute that’s not coupled to the external functionality that caused the discount—only the result. This sort of abstract design leads to a more open core behavior in your service and leads to less coupling.

#### 8.3.4. Jack of all trades, master of none

Everyone is familiar with the saying “jack of all trades, master of none,” which refers to a person who has a broad variety of skills but isn’t particularly proficient in any skill. In many ways, this adage is apropos for CRUD-based solutions. In [chapter 1](https://livebook.manning.com/book/reactive-application-development/chapter-1/ch01), we explored a monolithic shopping cart example comprised of several services. Digging deeper into that design by looking at an individual service, you see why this design is a jack and not a master ([figure 8.4](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig04)).

##### Figure 8.4. Detailed view of a monolithic order service; domains with hard dependencies on other domains in the same application instance

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig04_alt.jpg)

In [figure 8.4](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig04), all things related to orders are in a single monolithic application for the sake of transactionality, built with the traditional RDBMS techniques, resulting in subsystems being tied at the hip because they interact in some way. This application won’t scale, and if one piece breaks, everything is broken.

Within the order’s functionality in [figure 8.4](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig04) is a single domain aggregate representing Orders. Against this object, you action the three command behaviors (create, update, and delete) and the query behavior (read). The problem is that domain aggregates are designed to represent current state and aren’t particularly conducive to data projections (queries). Usually, when projecting data, you need some form of aggregation, as you want to see more than the order on the screen. As a result, you have to rely on building data access object (DAO) projections off the aggregate, which in turn require dynamic querying based on underlying SQL joins. This design is messy at best and becomes a continual point of refactoring for query optimization. In the end, you’re trying to use the aggregate for more than its design supports.

#### 8.3.5. Lack of behavior

A major, costly area of pain in typical monolithic applications (typically, CRUD-based) is the absence of data needed to derive intent. We discuss this topic in detail in [chapter 1](https://livebook.manning.com/book/reactive-application-development/chapter-1/ch01), but it’s important to review this topic, which is an area in which CQRS/ES shines. With CRUD, you have four behaviors: create, read, update, and delete. These behaviors are summary in nature, designed to modify current state models; they lack the historical deltas required for capturing purpose. This loss of intention has a significant effect on the value of business data, as you’re unable to determine user motive, which is paramount to understanding the user base. In using CRUD models, you effectively throw away massive amounts of data.

To better understand your users’ needs, you must be able to create a profile of their use habits so that you can accurately anticipate their future requirements. Creating this profile requires a detailed view of the actions that led up to a particular decision made by a user. Data capture at this level allows you to not only predict users’ future needs, but also answer questions that haven’t yet been asked (see [chapter 1](https://livebook.manning.com/book/reactive-application-development/chapter-1/ch01)). Additionally, you can aggregate these results across your entire user base to discover all kinds of patterns related to the context of your system. In essence, this approach provides the bounty from which you can do Big Data analysis.

Building a system of this variety requires component focus and specialization. Your system components must be masters, not jacks. CQRS/ES provides the philosophy and lays the groundwork for this type of application focus. To demonstrate the focus and specialization of CQRS/ES, devise a simple order tracking system. Define the domain aggregates and their behavior in the form of commands, and record those behaviors (if valid) in the form of historical events. You’ll see that you not only have access to the current state of the aggregate, but also can derive the state for any time in the past up to now. From this structure, you have a natural audit log, can infer motive, and have an architecture that’s designed to distribute.

#### 8.3.6. Order example: the order commands

This section looks at a familiar construct called an order. You’ve seen it before, which theoretically makes it easier to grasp what’s different in CQRS. If you want a more interesting domain, look at the flight domain in [chapter 5](https://livebook.manning.com/book/reactive-application-development/chapter-5/ch05) or read later chapters.

An order domain would look something like [figure 8.5](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig05).

##### Figure 8.5. The new order domain, separated from other domains

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig05_alt.jpg)

You see the order domain as a separate and atomic service. The only way to interact is via commands sent to the service by HTTP, but any other transport protocol can do the job. How does the order service interact with the other services in the top-right part of the figure? They also receive commands and also emit meaningful business events. These events may consume services that trigger their own events.

The order commands functionally mutate the order, but technically, no object mutation occurs, because immutable objects are first-class citizens of a reactive application. The commands should be as granular as possible, because the events derived from those commands are the building blocks of the event-driven nature of your applications. The ChangeOrder command, for example, would result in an OrderChanged event. This event provides no business insight into what happened to the order, so any other system that consumes order events must consume every OrderChanged event and interrogate it to determine whether the change of the order is of interest.

A better, more-expressive option is to break the events down as OrderShippingAddressChanged, OrderTotalChanged, and so on. The commands appear as follows:

-   CreateOrder()
-   ChangeBillingAddress()
-   ChangeShippingAddress()
-   PlaceOrder()

As we discuss in [chapter 5](https://livebook.manning.com/book/reactive-application-development/chapter-5/ch05) (which covers domain-driven design), Order is the aggregate root and contains OrderLine entities. All access to the order lines is through the root: Order.

Now that you have the idea of commands in place, the next section looks at a clean way to ensure that those commands are valid.

#### 8.3.7. Nonbreaking command validation

Nonbreaking validation is important to the responsiveness and usability of any microservice. Microservices that accept commands should validate those commands as an anticorruption layer and not allow pollution of the domain within. At the same time, you should embrace and expect failure; the clients of domains may not fully understand the ins and outs of the commands they’re sending. You can return a positive acknowledgment that the command is accepted and do your best to process it and generate resultant events, or you can create a convenient set of failed validations. The client can inspect these validations, make the repairs, and send the commands again. This technique is especially valuable when teams are working in parallel, microservices are evolving, and quick code fixes can be made to speed integration. It’s easier to deal with an in-your-face command failure than to try to read another team’s documentation as its domain evolves.

We don’t break, not ever, because we’re building a reactive-responsive application. Not breaking means guarding against nulls, empty strings, and numericals versus text, as well as more complex structures that you can validate against a known set of valid values.

The example in [listing 8.1](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08ex01) uses the Scalactic library. Implementation details may vary, but the contract is the important thing. Using REST over HTTP, you can express the contract for a command response as a 400-bad request containing a JavaScript Object Notation (JSON) list of the failed validations or a 204-accepted response, meaning that the command is considered valid and the system will make its best effort to process that command. This listing shows how you might implement some validation on an imaginary Order domain object.

##### Listing 8.1. Order validation

```
123456789101112131415161718192021222324252627282930313233343536373839404142434445464748495051525354555657585960616263646566676869707172737475767778798081828384858687888990919293949596
```

Using the code in [listing 8.1](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08ex01), you can try constructing an order a couple of times, once with valid attributes and again with a couple of invalid ones, as seen in the following listing.

##### Listing 8.2. Order validation demonstration

```
123456789101112131415
```

[Listing 8.2](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08ex02) shows embracing failure as a natural course of business via nonbreaking validation. It shows that clients are imperfect and that invalid data is expected but not allowed to pollute the domain. Instead, the code returns the validations cleanly to the client so that it may change its call properly.

#### 8.3.8. Conflict resolution

Now that you have a means to break down and distribute your applications in terms of queries and commands, you need to pay attention to a small price to be paid, which is data consistency. To understand the problem, consider the following example. An aircraft at 5,000 feet of altitude is issued a command by the tower to reduce that altitude by 2,000 feet. Then that same aircraft is issued that same command by arrivals, which last understood the aircraft’s altitude to be 5,000 feet. This sort of behavior could easily cause a catastrophe. To guard against this situation, include an expected aggregate version in each command. Only commands that match the expected version of the current aggregate version are processed. The problem is that an action (command) may be taking place on a domain aggregate based on a stale assumption about that aggregate. With a distributed application, special attention must be paid to aggregate versions. A version is established each time any part of the aggregate state is changed and that change results in one or more events being persisted to the data store, which may be called the event store on the command side. Each command is atomic from start to finish, meaning that if another command comes in, it’s handled only after the previous one has mutated (changed) the aggregate state.

In the next section, we show you how to view all this wonderful domain data, now carved up into independent and atomic services, by using the query side (the Q in CQRS).

### 8.4. The Q in CQRS

In this section, we show you how to address the impedance mismatch between the read data and the domain data in which it derives. The asynchronous nature of updating the query data provides a clean separation from the sources of the data, with no interruption of any runtime behaviors on the command or query side. But there’s a small price: this design is subject to inconsistency, in that there’s always some delay between the current state of the domain(s) and the query stores that depend on them. The guarantee is that if all activity stopped in a series of connected CQRS systems, all data would eventually look the same, becoming _eventually consistent_. These stores are sometimes called _projections_ of the domain. With CQRS alone, as opposed to CQRS with Event Sourcing, you don’t prescribe how the changes to the command side affect any given query sides.

#### 8.4.1. Impedance mismatch

When it comes to servicing clients of your systems, one of the first challenges you run into is object-relational impedance mismatch. [Figure 8.6](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig06) should be easy for non-engineers to follow. Hook up a small hose to a large one, and when the water flowing from the source meets the greater resistance (impedance, in this case) of the smaller hose, water is forced back and lost via leakage farther upstream. In the figure, the leakage is in the large hose’s connection to the faucet. The queries you need to view the data usually are a different shape from the data associated with the domain, which is also an impedance mismatch.

##### Figure 8.6. Impedance mismatch in water flow. A large hose feeds into a small one, resulting in undesired water flow in the opposite direction and water loss.

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig06.jpg)

The term _impedance mismatch_ comes from the electrical-engineering term _impedance matching_. In electronic-circuit design, _impedance_ is the opposition to the flow of energy from a given source. The idea is that as one component provides energy to another, the first component’s output has the same impedance as the second component’s input. As a result, the maximum transfer of power is achieved when the impedance of both circuits is the same.

This impedance mismatch, when it occurs between read and write concerns, can be a significant challenge in standard CRUD relational applications. Object-oriented domain models that employ CRUD rely on techniques that encapsulate and hide underlying properties and objects. The problem results in CRUD semantics, requiring object relational mapping (ORM), which must expose the underlying content to transform to a relational model, thus violating the encapsulation law of object-oriented programming (OOP). Although this problem isn’t the end of the world, it cripples your ability to distribute.

[Figure 8.7](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig07) shows an impedance mismatch in software.

##### Figure 8.7. Impedance mismatch in software

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig07_alt.jpg)

Using [figure 8.7](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig07) as a reference, suppose that you want to view the entire order status to get an overall picture of the order, who sold it, what was bought, what was shipped, what cash was collected, and so on. Because the domains are nicely broken apart, even to the point of being separate services, this Order to Cash view doesn’t exist—certainly not as a first-class citizen—within the diagram. The order status must be built on client demand by querying across all the necessary domains, which is an expensive operation that’s hard to keep tuned, let alone scale. What’s clearly missing is a query as a first-class citizen. This scenario demonstrates an impedance mismatch between the reality of the domains and the way that the clients desire to view them. As a large hose overwhelms a small one, trying to cobble together a client presentation in real time is problematic, leading not to water loss, but to loss in performance, risk, and errors.

#### 8.4.2. What is a query?

A CQRS _query_ is any read on a domain or a combination of domains. The associated domain data is considered to be a projection of the domain. A _projection_ is a sort of picture of the domain state at a point in time and is eventually consistent with the events in which it’s derived. The domain on the command side is rarely read directly but used to build up current state that clients can read quickly. In CQRS queries, no calculations are performed on reads; the data is always there, waiting to be read and indexed according to the client’s needs. Storage is cheap, so different client data requirements mean multiple projections, avoiding antipatterns such as the additions of secondary keys, or, worse, table scans to search within projections. Query projections can be as simple as the latest state of any domain or as complex as data from different domains. [Figure 8.8](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig08) shows the Order to Cash scenario, which also serves as a nice example of a query side.

##### Figure 8.8. Order to Cash query side, constantly fed command-side events to build itself

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig08_alt.jpg)

In [figure 8.8](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig08), you see the independent command sides of sales, customer, inventory, and billing asynchronously feeding the Order to Cash query module. By the time a client makes a call to view Order to Cash, all the data is built and waiting.

Now look at a simple projection of employee event data, assuming that the command side is using Event Sourcing (which we discuss in the next section). For this example, it’s enough to understand that the example has no single representation of an employee, but several events that tell the story of that employee from start to finish. In this story, an employee has been hired, had his pay grade changed, and was terminated. The example uses a _query projection_, which is a static view on the employee to show his current state at any given time. The domain doesn’t represent the current state of the employee; it shows a series of events occurring on that employee over time. Suppose that human resources required a view that shows all nonterminated (active) employees. Use JSON to express the contents of the event data for ease of reading. The reason is that different events of a single domain are usually written to the same event log/table, so that each one has a different footprint, and these differently formatted events don’t lend themselves to tabular presentation.

[Table 8.1](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08table01) shows events occurring on a single employee over time.

##### Table 8.1. Events that have occurred on the employee aggregate

<table><colgroup span="2"><col width="100"> <col width="580"></colgroup><tbody><tr><td>1</td><td><pre id="PLd0e22595">EmployeeHired({"id":"user1","firstName":"Sean",
"lastName":"Walsh","payGrade":1,"version":0})</pre></td></tr><tr><td>2</td><td><pre id="PLd0e22610">EmployeeNameChanged({"id":"user1","lastName":"Steven","version":1})</pre></td></tr><tr><td>3</td><td><pre id="PLd0e22625">EmployeePayGradeChanged({"id":"user1","payGrade":2,"version":2})</pre></td></tr><tr><td>4</td><td><pre id="PLd0e22640">EmployeeTerminated({"id":"terminationDate":"20150624","version":3})</pre></td></tr></tbody></table>

The read projection of this employee is indexed on the employee’s ID and contains all employees, although this example illustrates only one.

After the third event, EmployeePayGradeChanged, the projection appears as shown in [table 8.2](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08table02).

##### Table 8.2. The read projection of the employee _before_ the termination occurs

   
| 
ID

 | 

lastName

 | 

firstName

 | 

payGrade

 |
| --- | --- | --- | --- |
| user1 | Steven | Sean | 2 |

After all events through EmployeeTerminated, the query projection is empty because this user no longer exists in the eyes of the consumer. Note that the event logs still exist, even though the employee functionally is gone due to the termination. This data is valuable for use cases such as rehiring as well as for data retention for employment compliance purposes.

[Table 8.3](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08table03) shows the employee read projection after the termination.

##### Table 8.3. The read projection of the employee _after_ the termination occurs

   
| 
ID

 | 

lastName

 | 

firstName

 | 

payGrade

 |
| --- | --- | --- | --- |
| ...crickets |   |   |   |

Because this query is constantly being updated in the background, data is always hot, ready and waiting to be read. This asynchronous building of the read data in the background does away with much of the pain of impedance mismatch. In the next section, we review that pain.

#### 8.4.3. Dynamic queries not needed

In CQRS queries, data is aggregated asynchronously and is always ready and waiting. The difference in user experience is sometimes staggering with the reduced latency of the CQRS approach; the screens render in the blink of an eye. What you see is what you get. No magic happens at query time. The data is a certain size and shape in the database, and that size and shape are exactly what the client receives, although it’s marshaled to JSON and so on. This distinct design of the read side makes it easy to support, track down anomalies, and debug applications without digging through and debugging code.

Because we’re beating CRUD to death in this chapter, we’ll give it one last kick in the teeth. In the world of CRUD, queries become expensive to run and maintain, and sometimes require constant tuning. In the old, monolithic world, it’s common for clients to request aggregated domain data—domain data mashed up across multiple domain contexts. Attempting to get at this data at runtime is ridiculous; we know because we’ve done it, and it was a significant source of problems. This pattern requires presentation to domain tier adapters that make multiple calls to the back end domain to create aggregated data transfer objects (DTOs). Another pattern is to make SQL/NoSQL requests to the database directly from the presentation tier, bypassing the domain. If you design your systems this way, they’re closely coupled and can’t be distributed.

#### 8.4.4. Relational versus NoSQL

Most of us have used relational (SQL) databases at some point in our careers, and those databases have worked well. You can use SQL for Event Sourcing as well as the CQRS query side, but the concrete column model (in which every column and its type is known at design time and enumerated in a table) is inflexible compared with NoSQL. NoSQL allows the storage of documents containing any number of attributes that need not be known at design time, making the construction of CQRS/ES systems much faster and more agile than the construction of relational models containing strict sets of table fields. With NoSQL, you can serialize an object and its varying fields to a table. The table may also contain multiple types of objects of differing sizes and shapes, making it easy to store all the events of a domain type. With SQL, you have to know the columns ahead of time and constantly maintain migration scripts when your event or read stores change over time—lots of overhead that you can avoid by using any of the excellent open source NoSQL solutions available today.

So far in this chapter, you’ve learned about the distinct paths of CQRS and seen the distinct nature of the command and query sides. How they relate is where Event Sourcing is a natural fit and sits atop CQRS nicely, as we discuss in the next section.

### 8.5. Event Sourcing

_Event sourcing_ makes the source of all domain behavior the accumulation of the events that have taken place on that domain. No order is sitting in any database as part of the domain. The state of the order at any time is the accumulated playback of the events. It’s possible to use Event Sourcing independently of CQRS, although the two are complementary, as commands can result in events. We’ve seen large clients in which the adoption of the message-driven architecture is too ponderous, given the legacy systems and infrastructure; sometimes, compromises must be made. In this chapter, we concentrate on the CQRS/ES combination.

#### 8.5.1. What is an event?

An _event_ represents something meaningful in the domain that has occurred and is usually in the past tense, such as OrderCreated or OrderLineAdded. Events are immutable in that they represent something that happened at a particular point in time and are _persistent_, meaning that they’re stored in an event log such as a distributable database like Apache Cassandra. At any point in time, it’s possible to witness the state of the domain by the ordered accumulation of the events through that point in time. These events even appear with deletes, which are also additive, like any other event. A delete doesn’t occur as it would in CRUD; the event states that the domain object no longer exists at that point in time. Events are a cornerstone of being message-driven and are a great way to provide meaningful communication among microservices. Because it’s possible to have a large number of events on some domains, increasing the amount of time it takes to replay the events to derive current state, there’s the concept of the snapshot. We talk more later about replay and in-memory state as well as projections.

As you see in [figure 8.9](https://livebook.manning.com/book/reactive-application-development/chapter-8/ch08fig09), an _order_ is a sum of its events over time.

##### Figure 8.9. The events over time _are_ the order.

![](https://drek4537l1klr.cloudfront.net/devore/Figures/08fig09_alt.jpg)

##### Event Sourcing for the win!

In spring 2014, many technical dignitaries presented at the PhillyETE conference. On that occasion, we went to dinner with a group including people from Typesafe and Dr. Roland Kuhn, head of the Akka team. Greg Young walked in, still wearing his name tag, so we had the pleasure of meeting him and joining in some discussions.

We talked about CQRS and Akka, and about how we implemented command sourcing for some of our microservices. He blasted us for that practice for several reasons, most notably use cases involving one-time-only actions associated with commands such as a credit card transaction that would never be repeated with the playback of events. At this point, Roland and I challenged Greg, and by the end of the conversation, Roland and the Akka team decided to remove virtually all references to command sourcing from Akka Persistence documentation and to embrace Event Sourcing only. Pretty cool!

##### Snapshots

If you have a domain with millions of events, materializing state on that domain is nonperformant; the answer is snapshotting. _Snapshotting_, which involves separate storage from the event log, is the periodic saving of the state of the entire domain. _Replay_ is the full or partial rebuilding of state based on event history. On replay, the snapshot is the first thing retrieved; all events that occurred after the snapshot are replayed atop it.

Consider an energy-industry example. Various energy readings come into a domain representing a large industrial battery. Each second, a reading comes in for charge and discharge kilowatts. The state of the battery includes its kilowatt hour capacity. These events add and subtract from that state. Every day, a snapshot is saved to the database so that all events need not be replayed.

##### Replay and in-memory state

Replay is ordered retrieval of the snapshots and events stored in their storage mechanisms. Implementing replay is as easy as doing a read of the latest snapshot from one table, querying all the events that have taken place since the snapshot, and having the domain object apply those events one by one. Akka persistence has an elegant solution to this problem, as replay and snapshotting are built in. You model the domain, such as an order, as a persistent actor. When the actor is instantiated, Akka persistence automatically sends the snapshot and all the appropriate events to the actor as messages. From there, materializing the current state is a simple matter of building up private state within the actor from those messages. This state functions as a projection of the order and may be queried on in a performant, distributed manner.

Replay is an important aspect of Event Sourcing, in that there are no guarantees that all produced events will be received and successfully processed for every consumer. No true reliable messaging exists among so many moving parts—hence, the need for replay to rebuild application state when in doubt.

Another valuable application of replay is seeding data. Microservices normally can’t be guaranteed to have the same up and down time as the services with which they interact. Seeding provides the ability to catch up on all the events a service missed upon coming back online, as well as when the new service is spun up for the first time in an environment. Replay to the rescue!

#### 8.5.2. It’s all about behavior

With Event Sourcing, events mimic real life, and no behavior is lost. At some future point, it’s possible to answer questions that the business hasn’t even yet asked. Events capture the true in-sequence behavior in the domain that map to real life. When you look at the concept of CRUD, you see a nonbusiness set of operations: create, read, update, delete. These operations aren’t meaningful behavior; fortunately, you can leave them behind in favor of explicitly expressing your domain via events.

#### 8.5.3. Life beyond distributed transactions

What are you going to do without trusty ACID transactions, in which an update to an order could contain an update to a customer as a single unit? These transactions have been convenient to use to ensure multiple operations, using ORMs and paying a high penalty in terms of distribution. It turns out that these cross-boundary transactions aren’t necessary most of the time. You should carefully think through service-level agreements (SLAs) in the rare cases in which ordered transactions must occur as a single unit. In most cases, you can rely on eventual consistency across microservice boundaries in place of traditional transactions. Too often, strong consistency is the default in application design. When consistency is your go-to choice, you’re choosing it over availability, making this default expensive. Making this choice is like putting your hands up in the bank in case it gets robbed.

In situations that require stronger consistency, you can employ the Saga Pattern (discussed in [chapter 5](https://livebook.manning.com/book/reactive-application-development/chapter-5/ch05)). This pattern is implemented with the Process Manager Pattern in Akka. With this pattern, it’s possible to perform a sequence of commands or other messages across different contexts in the form of an orchestration. The pattern is a state machine, so each state has recovery logic in case of failure.

#### 8.5.4. The order example

In this section, you model your domain as a CQRS/ES command side by using an Akka persistent actor. You see how to ensure consistency of the order domain by using versioning, as well as examples of modeling the domain state, commands, and events.

Because domain state is a function of events occurring over time, it’s usually necessary to model the most current state of the domain exclusive of any consistency scheme (the guaranteed latest state.) You can approach this model by using a distributed database such as Cassandra and by using a _read projection_, which is a picture of the current state stored in the database. That state, however, is eventually consistent and doesn’t include the most current state guarantee.

To attain the most guaranteed view of current state, use Akka persistence to model your domain. When you use a single persistent actor in an actor cluster to represent a single domain object, that actor may contain the current state and can be the single point of access to that domain object. When you use actors in this way, you get caching for free because the actor can contain mutable internal variable(s) that it updates upon each new or replayed event. This mutable state is acceptable because it’s completely internal to the actor, and, therefore, thread-safe. The materialization of state inside the actor allows in-memory reads of that current domain state that may be used in orchestrations across domain objects. These actor implementations are also singletons (there is only ever one in the running system for any given aggregate), so there is no danger of contention while the aggregate is making a decision during command processing.

In the following listing, you see how to model state by using an Akka persistent actor. The code illustrates state management of an order aggregate modeled as a persistent actor.

##### Listing 8.3. Persistent order actor

```
123456789101112131415161718192021222324252627282930313233343536373839404142434445464748495051525354555657585960616263646566676869707172737475767778798081828384858687888990919293949596979899100101102103104105106107108109110111112113114115116117118119120121122123124125126127128129130131132133134135136137138139140141142143144145146147148149150151152153154155156157158159160
```

As you can see, Akka persistence provides an elegant way to handle all aspects of CQRS/ES domain design, including clean and nonbreaking validation and uncompromising data consistency. Next we look at consistency concerns that exist in the new world of CQRS/ES.

#### 8.5.5. Consistency revisited

With the disconnect that exists between the query and command sides, which are now separate concerns, both query and command sides are message-driven and eventually consistent using events. Consistency becomes an important aspect to consider. Consistency describes the guarantees of how distributed data is propagated across distributed systems and prescribes the manner in which data is seen across partition boundaries. The three types of consistency used in computing are _strong consistency_, _eventual consistency_, and _causal consistency_, and we talk about them in the following sections.

##### Strong consistency

_Strong consistency_ guarantees that all data is seen across all partitions, at the same time, and in the same sequence. Strong consistency is expensive and prevents you from distributing your applications. Surprisingly enough, strong consistency has been the go-to method of consistency in system development for some time. When you use database transactions with ORMs such as Hibernate, you get this level of consistency, but in most cases, no requirements dictate a need for this consistency. Think through any use of strong consistency carefully, because the only way to support this level of consistency is with close coupling of the related systems, which can lead to a monolith. We recommend that you never use strong consistency across distributed boundaries; the cost is too high.

##### Eventual consistency

_Eventual consistency_ is cheap and easy to implement, and you should strive to make it your consistency mode of choice. With this method, your data eventually becomes consistent across partitions, but the timing and ordering aren’t guaranteed. Make eventual consistency your first and (we hope) only consistency model. The typical flow of eventual consistency is for the CQRS command side to emit an event by using some bus, such as Akka cluster. On the read side of another microservice is an event listener actor that subscribes to that event over the Akka event stream. Upon receipt of any such message, that actor determines how the message affects the read projection(s) that it oversees and mutates those projections accordingly to match the latest state of the domain(s).

##### Causal consistency

_Causal consistency_ is the second-most-expensive consistency model, and you should avoid it whenever possible, although you’re likely to run into use cases that require it. Causal consistency guarantees that all partitions see the same data in the same sequence but not at the same point in time. Think of causal consistency as being an orchestration across your microservices/domains. In the next example, we show you an excellent pattern for this purpose, Process Manager.

#### 8.5.6. Retry patterns

In a distributed, reactive application, it’s difficult and most likely impossible to have 100% reliable messaging, but you can do your best to make messaging as reliable as possible by using durable messaging (Kafka or RabbitMQ) or the delivery semantics in Akka cluster.

Akka messaging semantics allow you to retry message delivery between actors until the message is known to be delivered to the recipient. The following list describes these semantics:

-   _At-least-once_ is the least expensive retry method, requiring the sending side to maintain storage of outstanding messages. With at-least-once, the sender always tries to redeliver a message until a confirmation of receipt is received from the recipient. With this method, it’s possible to deliver a message more than once, because the sender may be receiving but having difficulty sending the confirmation response.
-   _Exactly-once_ is the more expensive means of messaging, requiring storage on the sending and receiving sides. With exactly-once, a message is retried until it’s received, and the recipient is guaranteed to process it only once.

Always use at-least-once if you can.

In the next section, we discuss command sourcing versus Event Sourcing.

#### 8.5.7. Command sourcing versus Event Sourcing

_Command sourcing_ is logging commands as the source of record rather than the events, and _Event Sourcing_ is logging the events. Command sourcing creates a problem in that commands don’t always result in a change of domain state and are rejected. Why would you want to have a rejected command as a central part of the domain? It doesn’t make much sense unless you have a clear requirement to do so for audit purposes, and in such a case, the events should be logged as the source of record as well.

Another problem with command sourcing is that replay is an important part of CQRS/ES. Commands may trigger side effects such as a one-time credit card transaction, whereas the events occur after that logic and may be used to replay and rebuild domain state without any side effects. Replay of commands is complex and problematic, and should be avoided. Stick to Event Sourcing if you can.

### Summary

-   CQRS allows an easy way to be nonmonolithic, as reads are separated from writes, typically as separate applications.
-   CQRS provides an easy way to bulkhead applications.
-   Relational databases typically don’t scale, due to transactionality.
-   Event Sourcing provides a nice basis to be message-driven and ensures that no historical behavior is lost.
-   Akka provides an elegant CQRS and Event Sourcing solution out of the box.
-   Think carefully about your consistency models, and always lean toward the less-expensive option of eventual consistency.