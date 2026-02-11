# CQRS Documents by Greg Young

## Contents

1. [A Stereotypical Architecture](#a-stereotypical-architecture)
2. [Task Based User Interface](#task-based-user-interface)
3. [Command and Query Responsibility Segregation](#command-and-query-responsibility-segregation)
4. [Events as a Storage Mechanism](#events-as-a-storage-mechanism)
5. [Building an Event Storage](#building-an-event-storage)
6. [CQRS and Event Sourcing](#cqrs-and-event-sourcing)

---

## A Stereotypical Architecture

Before moving into architectures for Domain Driven Design based projects it is important to start off by analyzing what is generally considered to be the standard architecture that many try to apply to projects. We can from that point attempt to improve upon the stereotypical architecture in small rational steps while trying to minimize the cost in terms of productivity for each step towards a better architecture.

### Application Server

The stereotypical architecture is centered upon a backing data storage system. This system although typically a RDBMS does not have to be—it could just as easily be a key/value store, an object database, or even plain XML files. The important aspect of the backing store is that it is representing the current state of objects in the domain.

Above the backing data storage lies an Application Server. An area of logic, labeled as the domain, contains the business logic of the system. In this area validation and orchestration logic exists for the processing of requests given to the Application Server.

It is important to note that a "domain" is not necessary to achieve this architecture—one could also use other patterns such as Table Module or Transaction Script, with these only existing as Application Services.

Abstracting the "domain" one will find a facade known as Application Services. Application Services provide a simple interface to the domain and underlying data; they also limit coupling between the consumers of the domain and the domain itself.

On the outside of the Application Server sits some type of Remote Facade. This could be many things such as SOAP, custom TCP/IP, XML over HTTP, or even a person manually typing messages that arrive tied to the legs of pigeons. The Remote Facade may or may not be abstracted away from its underlying technology mechanism depending on the situation and tools that are involved.

### Client Interaction

The basic interaction of the client can be described as a DTO (Data Transfer Object) up/down interaction. Going through the lifecycle of an operation is the easiest way to show the functioning of the API:

1. A user goes to a screen, perhaps to edit a customer
2. The client sends a request to the remote facade for a DTO representing Customer #id
3. The Remote facade loads up the domain objects required, and maps the domain objects to a DTO that is then returned to the client
4. The client displays the information on the screen allowing the user to interact with it
5. The user completes editing and triggers a "Save"
6. The client packs the data back into a DTO and sends it to the Application Server
7. The Application Server maps the DTO back to the domain objects, verifies changes, and saves

Example DTO in XML format:

```xml
<Contact id="1234">
  <Name>Greg Young</Name>
  <Address>
    <Street>111 Some St.</Street>
    <City>Vernon</City>
    <State>CT</State>
    <Zip>06066</Zip>
    <Country>USA</Country>
  </Address>
</Contact>
```

### Analysis of the Stereotypical Architecture

#### Simplicity

The most likely property defining this architecture's popularity is that it is simple. One could teach a Junior developer how to interact with a system built using this architecture in a very short period of time. The architecture is completely generic—one could use it on every project. Because many people are doing it, if a team brings on a new member the new member will be intimately familiar with the general architecture of their system, lowering ramp up costs.

#### Tooling

Many frameworks exist for the optimization of time required to create systems utilizing this architecture. ORMs are the largest single example as they provide valuable services such as change tracking and transaction management with complex object graphs.

#### Domain Driven Design

**The application of Domain Driven Design is not possible in the above architecture** though many who are "practicing" Domain Driven Design use this architecture.

In the architecture above there are only four verbs: Create, Read, Update, and Delete (CRUD). Because the Remote Facade has a data oriented interface the Application Services must necessarily have the same interface.

This means that there are no other verbs within the domain. When however one talks with domain experts in an effort to refine an Ubiquitous Language, it is extremely rare that one ends up with a language that is focused on these four verbs.

There is a related well-known anti-pattern of domain modeling known as an "Anemic Model":

> "The basic symptom of an Anemic Domain Model is that at first blush it looks like the real thing. There are objects, many named after the nouns in the domain space, and these objects are connected with the rich relationships and structure that true domain models have. The catch comes when you look at the behavior, and you realize that there is hardly any behavior on these objects, making them little more than bags of getters and setters."
> — Martin Fowler

One cannot even create an Anemic Domain Model with this architecture as then all of the business logic would be in services. Here the services themselves are really just mapping DTOs to domain objects—there is no actual business logic in them. A large amount of business logic does not exist in the domain at all, nor in the Application Server; it may exist on the client but more likely it exists on either pieces of paper in a manual or in the heads of the people using the system.

**This is far worse than the creation of an anemic model; this is the creation of a glorified excel spreadsheet.**

#### Scaling

When one looks at this architecture in the context of scaling one will quickly notice that there is a large bottleneck: the data storage. When using a RDBMS (as 90%+ currently use) this becomes even more of a problem—most RDBMS are not horizontally scalable and vertically scaling becomes prohibitively expensive very quickly.

### Summary

The DTO up/down architecture employed on many projects is capable of being used for many applications and can offer many benefits in terms of simplicity for teams to work with. It cannot however be used with a Domain Driven Design based project; to attempt so will bring failure to your efforts at applying Domain Driven Design.

This architecture does however make a good baseline and the rest of this document will be focused on improving this architecture in incremental steps while attempting to limit or remove cost while adding business value at each additional step.

---

## Task Based User Interface

This chapter introduces the concept of a Task Based User Interface and compares it with a CRUD style user interface. It also shows the changes that occur within the Application Server when a more task oriented style is applied to its API.

One of the largest problems seen in "A Stereotypical Architecture" was that **the intent of the user was lost**. Because the client interacted by posting data-centric DTOs back and forth with the Application Server, the domain was unable to have any verbs in it. The domain had become a glorified abstraction of the data model. There were no behaviors—the behaviors that existed, existed in the client, on pieces of paper, or in the heads of the users of the software.

Many examples of such applications can be cited. Users have "workflow" information documented for them: "Go to screen xyz, edit foo to bar, then go to this other screen and edit xyz to abc." For many types of systems this type of workflow is fine. These systems are also generally low value in terms of the business. In an area that is sufficiently complex and high enough ROI in order to use Domain Driven Design, these types of workflows become unwieldy.

### Capturing Intent

Instead of simply sending the same DTO back up when the user is completed with their action, the client needs to send a message to the Application Server telling it to do something. It could be to "Complete a Sale", "Approve a Purchase Order", "Submit a Loan Application". Said simply: **the client needs to send a message to the Application Server to have it complete the task that the user would like to complete**. By telling the Application Server what the user would like to do, it is possible to know the intention of the user.

### Commands

The method through which the Application Server will be told what to do is through the use of a Command. A command is a simple object with a name of an operation and the data required to perform that operation. Many think of Commands as being Serializable Method Calls.

```csharp
public class DeactivateInventoryItemCommand {
    public readonly Guid InventoryItemId;
    public readonly string Comment;

    public DeactivateInventoryItemCommand(Guid id, string comment) {
        InventoryItemId = id;
        Comment = comment;
    }
}
```

**One important aspect of Commands is that they are always in the imperative tense**—they are telling the Application Server to do something. The linguistics with Commands are important. Placing Commands in the imperative tense linguistically shows that the Application Server is allowed to reject the Command; if it were not allowed to, it would be an Event.

It is quite common for developers to learn about Commands and to very quickly start creating Commands using vocabulary familiar to them such as "ChangeAddress", "CreateUser", or "DeleteClass". **This should be avoided as a default.** Instead a team should be focused on what the use case really is:

- Is it "ChangeAddress"? Is there a difference between "Correcting an Address" and "Relocating the Customer"? It likely will be if the domain in question is for a telephone company that sends the yellow pages to a customer when they move to a new location.
- Is it "CreateUser" or is it "RegisterUser"? "DeleteClass" or "DeregisterStudent"?

This process in naming can lead to great amounts of domain insight. To begin defining Commands, the best place to begin is in defining use cases, as generally a Command and a use case align.

It is also important to note that sometimes the only use case that exists for a portion of data is to "create", "edit", "update", "change", or "delete" it. All applications carry information that is simply supporting information. It is important though to not fall into the trap of mistaking places where there are use cases associated with intent for these CRUD-only places.

### User Interface

In order to build up Commands the User Interface will generally work a bit differently than in a DTO up/down system. Because the UI must build Command objects it needs to be designed in such a way that the user intent can be derived from the actions of the user.

The way to solve this is to lean more towards a **"Task Based User Interface"** also known as an "Inductive User Interface" in the Microsoft world. Microsoft identified three major problems with Deductive UIs:

1. **Users don't construct an adequate mental model of the product.** Most users don't acquire a mental model that is thorough and accurate enough to guide their navigation. These users aren't dumb—they are just very busy and overloaded with information.

2. **Even many long-time users never master common procedures.** Usability data indicates users focusing on the task at hand do not necessarily notice the procedure they are following and do not learn from the experience.

3. **Users must work hard to figure out each feature or screen.** Each feature or procedure is a frustrating, unwanted puzzle.

The basic idea behind a Task Based UI is that it's important to figure out how the users want to use the software and to make it guide them through those processes.

**Example: Deactivating an Inventory Item**

A typical deductive UI might have an editable data grid containing all of the inventory items with a dropdown for status. To deactivate an item the user would have to find the item, type in a comment, and change the dropdown to "deactivated."

A Task Based UI would take a different approach: show a list of inventory items with a link to "deactivate" next to each item. This link would take them to a screen that asks for a comment as to why they are deactivating the item. The intent of the user is clear and the software is guiding them through the process.

---

## Command and Query Responsibility Segregation

This chapter introduces the concept of Command and Query Responsibility Segregation. It will look at how the separation of roles in the system can lead towards a much more effective architecture.

### Origins

Command and Query Responsibility Segregation (CQRS) originated with Bertrand Meyer's Command and Query Separation Principle:

> It states that every method should either be a command that performs an action, or a query that returns data to the caller, but not both. In other words, asking a question should not change the answer.

**The fundamental difference is that in CQRS objects are split into two objects, one containing the Commands one containing the Queries.**

The pattern although not very interesting in and of itself becomes extremely interesting when viewed from an architectural point of view.

### Applying CQRS

A simple service to transform:

```
CustomerService
  void MakeCustomerPreferred(CustomerId)
  Customer GetCustomer(CustomerId)
  CustomerSet GetCustomersWithName(Name)
  CustomerSet GetPreferredCustomers()
  void ChangeCustomerLocale(CustomerId, NewLocale)
  void CreateCustomer(Customer)
  void EditCustomerDetails(CustomerDetails)
```

After applying CQRS:

```
CustomerWriteService
  void MakeCustomerPreferred(CustomerId)
  void ChangeCustomerLocale(CustomerId, NewLocale)
  void CreateCustomer(Customer)
  void EditCustomerDetails(CustomerDetails)

CustomerReadService
  Customer GetCustomer(CustomerId)
  CustomerSet GetCustomersWithName(Name)
  CustomerSet GetPreferredCustomers()
```

This separation enforces the notion that the Command side and the Query side have very different needs:

| Aspect | Command Side | Query Side |
|--------|--------------|------------|
| **Consistency** | Far easier to process transactions with consistent data | Most systems can be eventually consistent |
| **Data Storage** | Normalized (near 3NF) | Denormalized (1NF) to minimize joins |
| **Scalability** | Processes small percentage of transactions | Processes large percentage (often 2+ orders of magnitude more) |

**It is not possible to create an optimal solution for searching, reporting, and processing transactions utilizing a single model.**

### The Query Side

The Query side will only contain the methods for getting data. In the original architecture the building of DTOs was handled by projecting off of domain objects. This process can lead to a lot of pain:

- Large numbers of read methods on repositories often including paging or sorting information
- Getters exposing the internal state of domain objects in order to build DTOs
- Use of prefetch paths on the read use cases as they require more data to be loaded by the ORM
- Loading of multiple aggregate roots to build a DTO causes non-optimal querying

After CQRS has been applied there is a natural boundary. **It makes a lot of sense now to not use the domain to project DTOs.** Instead, introduce a new concept called a **"Thin Read Layer"**. This layer reads directly from the database and projects DTOs.

The Thin Read Layer need not be isolated from the database—it is not necessarily a bad thing to be tied to a database vendor from the read layer. It will not suffer from an impedance mismatch. It is connected directly to the data model, making queries much easier to optimize.

### The Command Side

The Command side remains very similar to the "Stereotypical Architecture." The main differences are:
- It now has a behavioral as opposed to a data centric contract (needed to actually use Domain Driven Design)
- It has had the reads separated out of it

Once the read layer has been separated, the domain will only focus on the processing of Commands. Domain objects suddenly no longer have a need to expose internal state, repositories have very few if any query methods aside from GetById, and a more behavioral focus can be had on Aggregate boundaries.

### Separated Data Models

By applying CQRS the concepts of Reads and Writes have been separated. It really begs the question of whether the two should exist reading the same data model or perhaps they can be treated as if they were two integrated systems.

There are many well known integration patterns between multiple data sources. The two distinct data sources allow the data models to be optimized to the task at hand. The Read side can be modeled in 1NF and the transactional model could be modeled in 3NF.

**The model that is best suited for integration is events**—events are a well known integration pattern and offer the best mechanism for model synchronization.

---

## Events as a Storage Mechanism

Most systems in production today rely on the storing of current state in order to process transactions. It has not always been like this.

Before the general acceptance of the RDBMS as the center of the architecture many systems did not store current state. This was especially true in high performance, mission critical, and/or highly secure systems. **In fact if we look at the inner workings of a RDBMS we will find that most RDBMSs themselves do not actually work by managing current state!**

### What is a Domain Event?

**An event is something that has happened in the past.**

All events should be represented as verbs in the past tense such as `CustomerRelocated`, `CargoShipped`, or `InventoryLossageRecorded`. They are things that have completed in the past.

It is absolutely imperative that events always be verbs in the past tense as they are part of the Ubiquitous Language. The introduction of the event makes the concept explicit and part of the Ubiquitous Language: relocating a customer does not just change some stuff, relocating a customer produces a `CustomerRelocatedEvent` which is explicitly defined within the language.

```csharp
public class InventoryItemDeactivatedEvent {
    public readonly Guid InventoryItemId;
    public readonly string Comment;

    public InventoryItemDeactivatedEvent(Guid id, string comment) {
        InventoryItemId = id;
        Comment = comment;
    }
}
```

Commands have an intent of asking the system to perform an operation whereas events are a recording of the action that occurred.

### Events as a Mechanism for Storage

When most people consider storage for an object they tend to think about it in a structural sense—a "Sale" that has "Line Items" and "Shipping Information." This is not however the only way that the problem can be conceptualized.

**The delta between two static states can always be defined.** More often than not this is left to be an implicit concept, usually relegated to a framework such as Hibernate or Entity Framework. Making these deltas explicit can be highly valuable both in terms of technical benefits and more importantly in business benefits.

The usage of such deltas can be seen in many mature business models. The canonical example is in accounting:

| Date | Comment | Change | Current Balance |
|------|---------|--------|-----------------|
| 1/1/2000 | Deposit from 1372 | +10000.00 | 10000.00 |
| 1/3/2000 | Check 1 | -4000.00 | 6000.00 |
| 1/4/2000 | Purchase Coffee | -3.00 | 5997.00 |
| 1/6/2000 | Purchase Internet | -5.00 | 5992.00 |
| 1/8/2000 | Deposit from 1373 | +1000.00 | 6992.00 |

Because all of the transactions associated with the account exist, they can be stepped through verifying the result. The "Current Balance" at any point can be derived either by looking at the "Current Balance" or by adding up all of the "Changes" since the beginning of time.

**It is mathematically equivalent to store the end of the equation or the equation that represents it.**

An Order viewed structurally:
```
Order → LineItems[] + ShippingInfo
```

The same Order viewed as events:
```
CartCreated → Added2SocksItem137 → Added4ShirtsItem354 → ShippingInfoAdded
```

By replaying through the events the object can be returned to the last known state. There is a structural representation of the object, but it exists only by replaying previous transactions. **Data is not persisted in a structure but as a series of transactions.**

One very interesting possibility: unlike when storing current state in a structural way, there is no coupling between the representation of current state in the domain and in storage. The representation of current state in the domain can vary without thought of the persistence mechanism.

### There is no Delete

It is not possible to jump into a time machine and say that an event never happened (i.e., delete a previous event). As such it is necessary to model a delete explicitly as a new transaction.

```
CartCreated → Added2Socks → Added4Shirts → Removed2Socks → ShippingInfoAdded
```

The two pairs of socks were added then later removed. The end state is equivalent to not having added the two pairs of socks. The data has not however been deleted—new data has been added to bring the object to the state as if the first event had not happened. This process is known as a **Reversal Transaction**.

There are also architectural benefits to not deleting data. The storage system becomes an additive only architecture. **Append-only architectures distribute more easily than updating architectures because there are far fewer locks to deal with.**

### Performance and Scalability

#### Partitioning

A very common performance optimization is Horizontal Partitioning (Sharding). One problem when attempting to use Horizontal Partitioning with a Relational Database: it is necessary to define the key with which the partitioning should operate.

**This problem goes away when using events.** Aggregate IDs are the only partition point in the system. No matter how many aggregates exist or how they may change structures, the Aggregate ID associated with events is the only partition point.

#### Saving Objects

When dealing with a stereotypical system utilizing relational storage it can be quite complex to figure out what has changed within the Aggregate. Most ORMs figure out changes by maintaining two copies of a given graph.

**In a system that is Domain Event centric, the aggregates are themselves tracking what has changed.** There is no complex process for comparing to another copy of a graph—instead simply ask the aggregate for its changes. The operation to ask for changes is far more efficient than having to figure out what has changed.

#### Loading Objects

Consider the work involved with loading a graph of objects in a stereotypical relational database backed system. Very often there are many queries that must be issued to build the aggregate. Many ORMs have introduced Lazy Loading to help minimize the latency cost of these queries.

When dealing with events as a storage mechanism things are quite different. **There is but one thing being stored: events.** Simply load all of the events for an Aggregate and replay them. There can only ever be a single query on the system—there is no need for things like Lazy Loading.

### Rolling Snapshots

Many would quickly point out that although it requires more queries in a relational system, when storing events there may be a huge number of events for some aggregates.

A **Rolling Snapshot** is a denormalization of the current state of an aggregate at a given point in time. It represents the state when all events to that point in time have been replayed.

```
Events:    [E1][E2][E3]...[E1000][E1001][E1002]...[E2000]
                          ↑                       ↑
Snapshots:           Snapshot@1000           Snapshot@2000

To load current state:
  1. Load latest snapshot (state at event 2000)
  2. Replay only events after snapshot (2001+)
```

Instead of reading from the beginning of time forward, read backwards putting the events onto a stack until either there were no more events left or a snapshot was found. The snapshot would then be applied and the events would be popped off the stack.

Introducing Rolling Snapshots allows control of the worst case when loading from events. The maximum number of events that would be processed can be tuned to optimize performance for the system in question.

**Rolling Snapshots are just a heuristic—conceptually the event stream is still viewed in its entirety.**

### Impedance Mismatch

The impedance mismatch between a domain model and a relational database has a large cost associated with it. A developer really needs to be intimate with both the relational model and the object oriented model, as well as the many subtle differences between them.

**There is not an impedance mismatch between events and the domain model.** The events are themselves a domain concept; the idea of replaying events to reach a given state is also a domain concept. The entire system becomes defined in domain terms.

### Business Value of the Event Log

Storing only current state only allows asking certain kinds of questions of the data. A vast majority of queries are focused on the "what"—labels to send customers mails, how much was sold in April, how many widgets are in the warehouse.

There are however other types of queries becoming more and more popular in business, they focus on the "how." Examples can commonly be seen in "Business Intelligence." Perhaps there is a correlation between people having done an action and their likelihood of purchasing some product?

**Example scenario:**

A domain expert believes there is a correlation between people having added then removed an item from their cart and their likelihood of responding to suggestions of that product by purchasing it at a later point.

**Team 1 (current state storage):** Plans to add tracking of items removed from carts. In the next iteration they build a report. The business receives a report with data only back to when tracking was implemented.

**Team 2 (event sourcing):** Adds the same tracking but also runs this handler from the beginning of the event log to back-populate all of the data from the time that the business started. **The report has data that dates back for years.**

The second team can do this because they have managed to store what the system actually did as opposed to what the current state of data is. **It is possible to go back and interpret the old data in new and interesting ways.**

As the events represent every action the system has undertaken, any possible model describing the system can be built from the events.

---

## Building an Event Storage

This chapter focuses on the implementation of an actual Event Storage.

### Structure

A basic Event Storage can be represented in a Relational Database utilizing only two tables.

**Events Table:**

| Column Name | Column Type |
|-------------|-------------|
| AggregateId | Guid |
| Data | Blob |
| Version | Int |

There will be one entry per event. The event itself is stored in the Data column using some form of serialization.

Most organizations would want to add columns such as the time that the change was made or context information (the user that initiated the change, the IP address, their permission level).

A version number is stored with each event. The version number is unique and sequential only within the context of a given aggregate. This is because Aggregate Root boundaries are consistency boundaries.

**Aggregates Table:**

| Column Name | Column Type |
|-------------|-------------|
| AggregateId | Guid |
| Type | Varchar |
| Version | Int |

The Aggregates table represents the aggregates currently in the system. Along with the identifier there is a denormalization of the current version number. This value is also used in the optimistic concurrency check.

### Operations

Event Storages are far simpler than most data storage mechanisms as they do not support general purpose querying. **An Event Storage at its simplest level has only two operations.**

**Operation 1: Get all events for an aggregate**

```sql
SELECT * FROM EVENTS WHERE AGGREGATEID='' ORDER BY VERSION
```

This is the only query that should be executed by a production system against the Event Storage.

**Operation 2: Write a set of events to an aggregate root**

```
Begin Transaction
  version = SELECT version from aggregates where AggregateId = ''
  if version is null
    Insert into aggregates
    version = 0
  end
  if expectedversion != version
    raise concurrency problem
  foreach event
    insert event with incremented version number
  update aggregate with last version number
End Transaction
```

The basic narrative:
1. Check if an aggregate exists with the identifier
2. If not, create it and consider the current version to be zero
3. Attempt an optimistic concurrency test—if the expected version does not match the actual version, raise a concurrency exception
4. Loop through the events being saved and insert them, incrementing the version number
5. Update the Aggregates table to the new current version number

**The interface:**

```csharp
public interface IEventStore {
    void SaveChanges(Guid AggregateId, int OriginatingVersion, IEnumerable<Event> events);
    IEnumerable<Event> GetEventsFor(Guid AggregateId);
}
```

That is it. All of the operations on the most basic Event Storage are completed.

### Rolling Snapshots

**Snapshots Table:**

| Column Name | Column Type |
|-------------|-------------|
| AggregateId | Guid |
| SerializedData | Blob |
| Version | Int |

A **Snapshotter** process sits behind the Event Storage and periodically queries for any Aggregates that need to have a snapshot taken because they have gone past the allowed number of events.

The process of creating a snapshot involves having the domain load up the current version of the Aggregate then take a snapshot of it. Once taken, it is saved back to the snapshot table.

Many use the default serialization package with good results though the **Memento pattern** is quite useful when dealing with snapshots. The Memento pattern better insulates the domain over time as the structure of the domain objects change.

### Event Storage as a Queue

Very often events are not only saved but also published to a queue where they are dispatched asynchronously to listeners. An issue that exists with many systems publishing events is that they require a **two-phase commit** between the storage and the queue.

If a failure were to happen between committing the storage and committing the queue, listeners would be out of sync with the producer.

**Solution: Use the Event Storage as a Queue**

Add a sequence number to the Events table:

| Column Name | Column Type |
|-------------|-------------|
| AggregateId | Guid |
| Data | Blob |
| SequenceNumber | Long |
| Version | Int |

The database ensures that values of sequence number are unique and incrementing (auto-incrementing type). A secondary process can chase the Events table, publishing the events off to the queue. The chasing process simply stores the value of the sequence number of the last event it processed.

The work has been taken off of the initial processing in a known safe way. The publish can happen asynchronously to the actual write. This lowers the latency of completing the initial operation and limits the number of disk writes in the processing of the initial request to one.

---

## CQRS and Event Sourcing

CQRS and Event Sourcing become most interesting when combined together.

**CQRS allows Event Sourcing to be used as the data storage mechanism for the domain.** One of the largest issues when using Event Sourcing is that you cannot ask the system a query such as "Give me all users whose first names are 'Greg'." This is due to not having a representation of current state. With CQRS the only query that exists within the domain is GetById which is supported with Event Sourcing.

**Event Sourcing is also very important when building out a non-trivial CQRS based system.** Maintaining separate relational models—one for read and the other for write—is quite costly. It becomes especially costly when you factor in that there is also an event model in order to synchronize the two. With Event Sourcing the event model is also the persistence model on the Write side. This drastically lowers costs of development as no conversion between the models is needed.

### Cost Analysis

| Component | Stereotypical Architecture | CQRS + Event Sourcing |
|-----------|---------------------------|----------------------|
| Client | Identical work | Identical work |
| Queries | Built off domain model | Thin Read Layer projecting DTOs (equal or less work) |
| Domain + Persistence | ORM with Impedance Mismatch | Events with no Impedance Mismatch |
| Read Model | N/A | Events → Relational (smaller mismatch) |

The Impedance Mismatch between events and a Relational Model is much smaller than the mismatch between an Object Model and a Relational Model. The Event Model does not have structure—it is representing actions that should be taken within the Relational Model.

**The two architectures have roughly the same amount of work. It's not a lot more work or a lot less work—it's just different work.** The event-based model also offers all of the benefits discussed in "Events."

### Integration

With the stereotypical architecture no integration has yet been supported, except perhaps integration through the database (a well established anti-pattern). Integration is viewed as an afterthought.

**With the CQRS and Event Sourcing based model, integration has been thought of since the very first use case.** The Read side needs to integrate and represent what is occurring on the Write Side—it is an integration point. The integration model is "production ready" throughout the initial building of the system.

The event based integration model is also known to be complete as all behaviors within the system have events. **If the system is capable of doing something, it is by definition automatically integrated.**

### Differences in Work Habits

#### Parallelization

Instead of working in vertical slices, the team can work on three concurrent vertical slices: the client, the domain, and the read model. This allows for better scaling of the number of developers on a project since they are isolated from each other.

```
Client  →  Consumes DTOs, produces Commands
Domain  →  Consumes Commands, produces Events
Read    →  Consumes Events, produces DTOs
```

It would be reasonable to nearly triple a team size without introducing a larger amount of conflict due to not needing to introduce more communication. This can be extremely beneficial when time to market is important.

#### All Developers are not Created Equally

The points of decoupling support the specialization of teams. For the domain, the best candidate is a person with large amounts of business knowledge, technical proficiency, and soft skills to talk with domain experts. When dealing with the read model and the generation of DTOs, the requirements are different—it is relatively straightforward work.

#### Outsourcing

The Read Model is an ideal area of the system to outsource. The contracts for the Read Model as well as specifications for how it works are quite concrete and easily described. Little business knowledge is needed.

The Domain Model on the other hand will not work at all if outsourced. The developers need large amounts of communication with domain experts and will benefit greatly by having initial domain knowledge. **These developers are best kept locally within the team and should be highly valued.**

#### Specialization

The "best" developers work with the domain. When working with a vertical slice, anecdotal evidence suggests they spend roughly 20-30% of their time in this endeavor.

**With the CQRS architecture, the team of developers working with the domain spend 80+% of their time working with the domain and interacting with Domain Experts.** The developers have no concern for how the data model is persisted, or what data needs to be displayed to users. They focus on the use cases of the system. They need only know Commands and Events.

This specialization frees them to engage in the far more important activities of reaching a good model and a highly descriptive Ubiquitous Language with the Domain Experts.

---

## Works Cited

- Ambler, S. W. "The Object Relational Mismatch." agiledata.org
- Evans, E. (2001). *Domain Driven Design*. Addison Wesley.
- Fowler, M. "Domain Event." martinfowler.com/eaaDev/DomainEvent.html
- Fowler, M. "Anemic Domain Model." martinfowler.com/bliki/AnemicDomainModel.html
- Jill Nicola, M. M. (2002). *Streamlined Object Modelling*. Prentice Hall.
- Microsoft Corporation. (2001). "Microsoft Inductive User Interface Guidelines." MSDN.
- Wikipedia. "Object-Relational Impedance Mismatch."
- Wikipedia. "You ain't gonna need it."
