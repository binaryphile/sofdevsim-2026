-   How to persist your domain models in a database
-   The two primary models of persistence: the CRUD model and the event-sourced model
-   The pros and cons of both models and how to select one for your domain model architecture
-   An almost complete implementation of an event-sourced domain model
-   How to implement a CRUD-based model functionally using a functional-to-relational framework

This chapter presents a different aspect of domain modeling: how to persist your domain model in an underlying database so that the storage is reliable, replayable, and queryable. You’ve likely heard about storage being reliable, thereby preventing data loss, and queryable, offering APIs so that you can use algebra (such as relational algebra) to query data from your database. This chapter focuses on two more aspects of persistence: traceability and replayability. In these cases, the data store [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)also serves as an audit log and keeps a complete trace of all actions that have taken place on your data model. Just imagine the business value of such a storage mechanism that offers built-in traceability and auditability of your domain and data model!

[Figure 8.1](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig01) shows a schematic of the chapter content. This guide can help you selectively choose your topics as you sail through the chapter.

##### Figure 8.1. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The progression of this chapter’s sections

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig01_alt.jpg)

At the end of the chapter, you should be in a position to appreciate how to use proper abstractions to make your domain model responsive to users and resilient to failures.

## [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)8.1. Persistence of domain models

[Chapter 3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-3/ch03) presented the three main stages in the lifecycle of a domain object. Any domain object gets _created_ from its components, participates in the various roles that it’s supposed to _collaborate_ in, and finally gets _saved_ into a persistent store. In all discussions so far, we’ve abstracted the concern of persistence in the form of a repository, often with APIs that handle creating, querying, updating, and deleting domain elements. Here’s the form of the API that we discussed so far:

```
trait AccountService[Account, Amount, Balance] {
  def open(no: String, name: String, rate: Option[BigDecimal],
           openingDate: Option[Date], accountType: AccountType)

           : AccountRepository => NonEmptyList[String] \/ Account
  //..
}
```

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)This API makes it explicit that you inject a repository as an external artifact in order for open to operate correctly. Now is the time to ask what form of AccountRepository you should use in your model and deploy in production.

In most usage patterns, the common form that a repository takes is that of a relational database management system. All domain behaviors ultimately map to one or more of Create, Retrieve, Update, and Delete (CRUD) operations. In many applications, this is an acceptable model, and lots of successful deployments use this architecture of data modeling underneath a complex domain model.

The biggest advantage of using an RDBMS-based CRUD model as a repository is familiarity among developers. SQL is one of the most commonly used languages, and we also have a cottage industry of mapping frameworks that manages<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn01" id="fn01b">1</a>]</sup> the impedance mismatch between the OO or functional domain model and the underlying relational model.

But the CRUD style of persistence model has at least a couple of disadvantages. An RDBMS often has a single point of failure and is extremely difficult to scale beyond a certain volume of data, especially in the face of high write loads. Concurrent writes with serializable ACID<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn02" id="fn02b">2</a>]</sup> transaction semantics don’t scale beyond a single node. This calls for not only a different way of thinking of your data model, but also an entirely different paradigm to think of the consistency semantics of your domain model.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn03" id="fn03b">3</a>]</sup> I will discuss how using alternative models of persistence can address the scalability problem.

To understand the other issue that plagues the CRUD model of persistence, especially with an underlying functional domain model, let’s consider an example model of a CheckingAccount that we discussed in [chapter 3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-3/ch03):

```
case class CheckingAccount (no: String, name: String,
  dateOfOpen: Option[Date], dateOfClose: Option[Date] = None,
  balance: Balance = Balance()) extends Account
```

CheckingAccount is an algebraic data type that’s immutable, and you can never do any in-place mutation on an instance of the class. But you already knew this, right? You’ve seen the benefits of immutability and the dark corners of shared mutability. That’s one of the essences of pure functional modeling that we’ve been talking about for the last seven chapters of this book. Now let’s translate this operation to the CRUD-based persistence model. [Figure 8.2](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig02) shows the result when you perform a debit operation on the account.

##### Figure 8.2. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Updates in a CRUD-based persistence model using an RDBMS. It stores the current snapshot, losing history of all changes.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig02_alt.jpg)

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)You have a table, CheckingAccount, that stores all data related to the account. Every time you process an instruction through a domain behavior that impacts the balance of the account, the field balance is updated _in place_. Hence after a series of updates, you have only the value that shows the latest balance of the account. You’ve lost important data! You’ve lost the entire sequence of actions that has led to the current balance of $1,050.

This means you won’t be able to do the following anymore from your underlying persistent data:

-   Query for the history of changes (unless you mine through the audit log that the database stores, which itself is mind-bogglingly complex in nature)
-   Roll back your system to sometime in the past for debugging
-   Start the system afresh from zero and replay all transactions that have occurred to date, to bring it back to the present

In summary, you’ve lost the traceability of your system. You have only the current snapshot of how it looks _now_. _Your data model is the shared mutable state_. This chapter presents techniques that allow you to model your repository in such a way that includes time as a separate dimension. The system will have the complete record of changes since inception and in a chronological fashion that offers complete traceability and auditability of the model. Because you’ve become an expert in functional programming, you can say that the current state of your model is a _fold_ over all previous states, with the start state being the initial.

## [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)8.2. Separation of concerns

When I discuss models of persistence in this chapter, I frequently draw analogies to aspects of functional domain models and remind us of the lessons we learned there. Let’s see whether we can apply some of those techniques at the persistence level as well [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)and enjoy similar benefits. Separation of concerns is a cross-cutting quality that we consider to be a desirable attribute of any system we design. In my discussion on domain model design, you saw how functional programming separates the _what_ from the _how_ of your model. You saw how we focused on designing abstractions such as free monads in [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05) that separate the definition of your abstraction from the interpretation. When we talk about persistence of domain models, the two aspects that demand a clear separation are the _reads_ and the _writes_. In this section, you’ll learn how to achieve this separation and get better modularity of your data model.

### 8.2.1. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The read and write models of persistence

A user who wants to view a model usually likes to have a business view of it. For example, a user who views an account wants to see all attributes as per the format and guidelines that the business domain mandates. This underlying Account aggregate may consist of multiple entities, possibly normalized for storage. But as a user, I’d like to see a denormalized view based on the _context_. If I’m an account holder in the bank and want to view the balance of my account, I should be able to get to the details that I need for online banking purposes. Another user of the system may be interested in a snapshot purely from an accounting perspective; maybe she’s interested only in the attributes of the account that she needs to send to an ERP system downstream for accounting purposes. Here we’re speaking of two views of the same underlying aggregate, one for the online banking context and the other for the accounting context. And this is again completely independent of the underlying storage model. [Figure 8.3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig03) gives an overview of this separation of the two models.

##### Figure 8.3. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Separating the read models from the write model. A single write model feeds the read models. The feed depends on the bounded context and the information that the specific read model requires.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig03.jpg)

So you’ve achieved one level of separation in your underlying data model. All reads will be served from the read models, and all writes will be done through the write [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)model. From the system-engineering point of view, if you think a bit, you’ll see that this separation makes complete sense. For reads, you’d like to have underlying data represented more closely to the queries and reports that they serve. This is the denormalized form that avoids joins in relational queries and just the opposite of what you’d like to do for writes. So by separating the two concerns, you get more appropriate models at the read and write levels.

Reads are easier to scale. In a relational database, you can add read slaves depending on your load and get almost linear scalability. Writes, on the other hand, are a completely different beast altogether. You have all the problems of shared mutable state if you employ a CRUD-based write model. The difference with this domain model is that here the RDBMS manages the state for you. But being managed by someone else doesn’t imply that the problems disappear; they just move down one level. In the course of this discussion, I’ll try to address this issue and come up with alternate models that don’t have to deal with the mutation of persistent data.

The architecture of [figure 8.3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig03) has quite a few issues that we haven’t yet addressed:

-   How will the read models be populated from the write model?
-   Who is responsible for generating the read models and fixing the protocols that will generate them?
-   What kind of scalability concerns does the preceding architecture address?

We’ll address all of these concerns shortly. But first let’s take a look at a pattern at the domain-model level that segregates domain behaviors that read data (_queries_) from the ones that update aggregates and application state (which we call _commands_), and that makes good use of the underlying read and write persistence models.

### 8.2.2. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Command Query Responsibility Segregation

Let’s get back to our domain model of personal banking and look at some of its core behaviors. A few examples are opening an account with the bank, depositing money into my account, withdrawing money from my account, and checking the balance. The most obvious difference in these behaviors from a modeling perspective is that the first three of them involve changing the state of the model, while checking the balance is a read operation. To implement these behaviors, you’ll work with the Account aggregate that will be impacted as follows with these operations:

-   _Open a new account_—Creates a new aggregate in memory and updates the underlying persistent state
-   _Deposit money into an existing account_—Updates balance of an existing account aggregate
-   _Withdraw money from an existing account_—Updates balance of an existing account or raises an exception in case of insufficient funds to withdraw
-   _Check the balance_—Returns the balance from an account; no application state changes

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Of these operations, the ones that change the application state are called _commands_. Commands take part in domain validation, updating in-memory application state and persistence at the database level. In summary, commands impact the write model of our architecture in [figure 8.3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig03). The operation of checking the account balance is a read or a _query_ that can be served well through the read model of our architecture.

Command Query Responsibility Segregation (CQRS) is a pattern that encourages separate domain model interfaces for dealing with commands and queries.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn04" id="fn04b">4</a>]</sup> It provides a separation of reads and writes at the domain model and the persistence model. [Figure 8.4](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig04) illustrates the basic CQRS architecture.

##### Figure 8.4. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Commands update the application state and persist in the write model. The queries, on the other hand, use the read model.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig04_alt.jpg)

As you can see, the CQRS pattern applied at the domain-model level nicely complements the idea of its dual model at the persistence level. [Figure 8.4](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig04) is self-explanatory, but here are a few more things that you need to be aware of while applying the CQRS pattern. Strictly speaking, these aren’t drawbacks of the pattern, but you need to take [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)a careful look and decide whether any of these constraints can be a potential bottleneck in your system:

-   It’s not mandatory that you have two physically separate databases for read and write models. You can use a normalized relational schema for the write model and a bunch of views for the read model—all part of the same database.
-   Depending on your application requirement, you may have multiple read models with a single underlying write model. Just keep in mind that the read models need to serve queries. Hence the schema of a read model needs to be as close to the query views as possible. This need not necessarily be relational; your write model can be relational, whereas the read model can be served from something completely different (for example, a graph database).
-   In some cases, the read models need to be explicitly synchronized with the write model. This can be done either using a publish-subscribe mechanism between the two models or through some explicitly implemented periodic jobs. Either way, a window of inconsistency may arise between your write and read models. Your application needs to handle or live with this eventual consistency.

So now you have separate write and read models at the database level and commands and queries at the domain-model level, with clear and unambiguous lines of interaction defined between them. The next section introduces another pattern that nicely complements CQRS and addresses some of the drawbacks of the CRUD model.

## [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)8.3. Event sourcing (events as the ground truth)

In the CQRS architecture, the write model faces all online transactions that can potentially involve mutation of shared data. With a typical CRUD-based data-modeling approach, this has all the sufferings and drawbacks that we discussed in [section 8.2](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08lev1sec2). Updates (the _U_ in CRUD) involve mutation of shared state implemented through locking and pose a serious concern in scalability of the model, especially in the face of high write loads. And as you saw in [section 8.2](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08lev1sec2) by in-place updating of data, you lose information of historical changes that take place in your system.

_Event sourcing_ is a pattern that models database writes as streams of events.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn05" id="fn05b">5</a>]</sup> Instead of a snapshot of an aggregate being stored in the database as in the CRUD model, you store all changes to the aggregate as an event stream. You have the entire sequence of operations that has transformed the aggregate from inception to the current state. With this information, you can roll back and obtain a snapshot of the aggregate at any time in the past and again come back to the current snapshot. This gives complete traceability of the entire system for purposes such as auditing and debugging. [Figure 8.5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig05) describes how to combine CQRS and event sourcing as the foundation of our persistence model.

##### Figure 8.5. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Events as the source of truth that get folded over to generate the current snapshot of the system. This generates the read model used by queries.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig05_alt.jpg)

### 8.3.1. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Commands and events in an event-sourced domain model

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)As [figure 8.5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig05) illustrates, here’s how the domain model behaviors interplay with the underlying persistence model, using events as the primary source of truth:

-   The user executes a command (for example, opening an account or making a transfer).
-   The command needs to either create a new aggregate or update the state of an existing one.
-   In either case, it builds an aggregate, does some domain validations, and executes some actions that may also include side effects. If things go well, it generates an event and adds it to the write model. We call this a _domain event_, which we discuss in more detail in the following subsection. Note we don’t update anything in the write model; it’s essentially an event stream that grows sequentially.
-   Adding an entry in the event log can create notifications for interested subscribers. They subscribe to those events and update their read models.

In summary, events are the focal points of interest in an event-sourced model. They persist as the source of truth and get published downstream for interested parties updating read models. Let’s discuss in more detail what domain events look like and the exact role that they play in our proposed model.

#### [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Domain events

A _domain event_ is a record of domain activity that has happened in the past. It’s extremely important to realize that an event is generated only _after_ some activity has [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)already happened in the system. So an event can’t be mutated; otherwise, you’d change the history of the system. Here are a few defining characteristics of an event in a domain model:

-   _Ubiquitous language_—An event must have a name that’s part of the domain vocabulary (just like any other domain-model artifact). For example, to indicate that an account has been opened for a customer, you can name the event Account-Opened. Or if you have the proper namespaces for modules, you can have a module named AccountService and have an event named Opened within it. Also note that the name of an event needs to be in the past tense, because it models something that has already happened.
-   _Triggered_—An event may be generated from execution of commands or from processing of other events.
-   _Publish-subscribe_—Interested parties can subscribe to specific event streams and update their respective read models.
-   _Immutable_—An event being immutable is usually modeled as an algebraic data type. You’ll look at the exact implementation when I discuss one of our use cases.
-   _Timestamped_—An event is an occurrence at a specific point in time. Any data structure that models an event has to have a timestamp as part of it.

[Figure 8.6](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig06) illustrates an example use case; specific commands are executed as part of account services, and events are logged into the write model. This simple use case explains the basic concepts of how commands can generate events that get logged into the write model and help subscribers update their read models. I’ve deliberately [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)chosen a familiar use case so that you can relate to the examples presented in earlier chapters.

##### Figure 8.6. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Commands generate events that get logged in the event log (write model). Parties can subscribe to the event stream and update read models. Note the naming of domain events and how they belong to the domain vocabulary.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig06_alt.jpg)

#### [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Command handlers

_Commands_ are actions that can loosely be said to be the dual of events. At the implementation level, we talk about command handlers when we abstract the domain behaviors that a command could execute. These include creating or updating aggregates, performing business validations, adding to the write model, and doing any amount of interaction with external systems, such as sending email or pushing messages to the message queue. You know how to abstract side effects functionally, and you’ll use the same techniques in implementing command handlers. Command handlers are nothing more than abstractions that need to be evaluated for performing a bunch of actions leading to events being generated and added to the stream (write model). [Figure 8.7](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig07) describes the flow of actions within a command handler that implements a function to debit an amount from a customer account.

##### Figure 8.7. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)A command handler that debits from a customer account. Note the three main stages involved in the flow within the command handler.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig07_alt.jpg)

### 8.3.2. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Implementing CQRS and event sourcing

As mentioned earlier in this chapter, event sourcing and CQRS are often used together as a pattern of persistence in domain models. [Figure 8.6](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig06) shows a sample flow that’s executed when a command is processed and events are generated. But quite a few subtle points make the implementation model not so simple, especially if you want to keep the model functional and referentially transparent. This section presents some of these issues, followed by sample implementations in the upcoming sections.

Just as a recap, here’s the flow of commands and events through a CQRS- and ES-powered domain model, with added details:

-   You fork a command through a service interface (which can be UI or REST endpoints or any other agent that invokes a domain service).
-   [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)If the command needs to work on a new instance of an aggregate (for example, creation of a new account), it checks from the event store that the instance hasn’t already been created. Bail out if any such validation fails.
-   If the command needs to work with an already existing aggregate (for example, a balance update), it needs to create the instance either from the event store through event replays or get the latest version of it from the snapshot store. Note that the snapshot store is an optional optimization that stores the current snapshot of every aggregate through periodic replays from the event store. But for large domain models it’s always used for performance considerations.
-   The command processor then does all business validations on the aggregate and the input parameters and goes on processing the command. Note that command processing involves two major steps:
    -   Perform domain logic that changes the states of aggregates such as setting the closing date of an account in case of an account close command, and may also involve side effects (for example, sending out email or interacting with third-party services).
    -   Generate events that get logged into the event store. In response, subscribers to specific events update their read models.
-   If you need to rebuild specific aggregates from scratch, you have to replay events from the event log. This may sound simple, but remember, replaying events requires only re-creating the state and _not_ repeating any side effects that the commands may have performed earlier. I’ll discuss in the implementation model how to decouple side effects (which only command handlers need to execute) from the state changes (which both command and event handlers need to execute).

#### [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Batching commands

We’ve always valued compositionality; why should we forego it for commands? After all, commands are APIs, and we’d like to make them compositional. You should be able to compose larger commands out of smaller ones, just as we discussed composition of APIs in earlier chapters. Here’s an example from our domain. Suppose you have individual commands for debit and credit of accounts. Composing a transfer command by combining the two should be possible:

```
def transfer(from: String, to: String,
  amount: Amount): Command[Unit] = for {
    _ <- debit(from, amount)
    _ <- credit(to, amount)
  } yield ()
```

Remember, you got similar compositionality with our domain service APIs in [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05). Do you recollect the technique you used? Yes! Free monads.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn06" id="fn06b">6</a>]</sup> They’re ready to [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)make a comeback in our implementation of event-sourced domain models. You’ll define commands as free monads over events so that you can combine commands monadically to build the algebra of our model behavior. You can learn all the details in the next section.

#### [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Handling side effects

One extremely important issue to solve if you want to keep your model pure and referentially transparent is how to handle side effects. Execution of command handlers will have side effects, and you’d like to decouple them from the state-changing APIs. Event handlers when replayed need to change states, and this can’t induce side effects. You wouldn’t want your customers to receive email every time you replay events within your model.

After you use free monads for command composition, the interpreter of the free monad can handle all your side effects. And because commands are the free monads, the side effects are limited to execution of commands only. You have to extract the state-change APIs as separate functions that can be reused separately by the command and event handlers. This is just an overview of the strategy that you’ll adopt. The implementation in the next section gives you all the details.

## [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)8.4. Implementing an event-sourced domain model (functionally)

This section covers some of the implementation aspects of an event-sourced domain model using the domain of personal banking. We use some of the use cases from earlier chapters so that you can relate to the functionality and think in terms of persistence of those model elements using the paradigm of event sourcing. This is by no means a production-ready, event-sourced domain model implementation; we’ll simplify wherever we can. The idea is to give you some food for thought for a functional implementation and highlight some of the relevant issues that may arise.

As hinted in the previous section, you’ll use free monads to implement event sourcing. Not that this is the only way to implement event sourcing (many other alternative techniques exist as well), even with functional programming. But I choose this approach mainly because it offers a nice separation of the algebra of events from their interpretation. You can keep on building sequences of commands that are pure, and only when you have the final stack of commands can you submit for evaluation through the interpreter of the algebra. This makes the whole model more compositional and restricts side effects only inside the interpreter—just what the doctor for FP ordered.

Here’s our general strategy of implementation:

-   Define the algebra of events by defining each event as an algebraic data type that can be connected to each other. You saw how to do this in [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05), when you implemented a free-monad-based account repository.
-   Make a free monad out of the event; this monad is the command. So now you can combine commands by using for-comprehensions and build larger commands. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)This is phase 1 of our implementation that builds up our command stack as a pure data type with no denotation.
-   In phase 2 of our implementation, you execute actions corresponding to each of the commands. Here you design an interpreter that takes the whole command stack, traverses it, and does a pattern match on the events associated with each command. For every event, you implement the action that it’s supposed to execute for processing that command.

### 8.4.1. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Events as first-class entities

A domain event is a first-class entity in the entire paradigm of event sourcing. The entire model is based around domain events, the model history is stored as event streams, and model (re)generation is done via event replays. It’s no surprise that we reflect the same fact in our domain model implementation. The following listing provides the basic Event abstraction and the algebra for the events that you plan to handle in our AccountService implementation.

##### Listing 8.1. Algebra of events

```
import org.joda.time.DateTime
import cqrs.service._
import common._

trait Event[A] {                                                         #1
  def at: DateTime                                                       #2
}

case class Opened(no: String, name: String,
  openingDate: Option[DateTime],
  at: DateTime = today) extends Event[Account]

case class Closed(no: String, closeDate: Option[DateTime],
  at: DateTime = today) extends Event[Account]

case class Debited(no: String, amount: Amount, at: DateTime = today)
  extends Event[Account]

case class Credited(no: String, amount: Amount, at: DateTime = today)
  extends Event[Account]                                                 #3
```

Here, all events are named in the past tense, indicating that they’ve already occurred. Hence they’re immutable, as can be inferred from the algebraic data types defined for [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)each of them. And finally, the names of the events are from the domain vocabulary, which is perhaps something that you’ve learned to accept as the norm by now.

Now that you know what each of the events looks like in delivering an account service to the customer, let’s step back a bit and think about how these events are generated in the first place. Commands get executed, and as discussed in [section 8.3.2](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08lev2sec4), the states of the aggregates change in the course of those actions. And if a command executes successfully, you generate an event. In the same section, you also saw that the state of an aggregate may change in the course of event replays. Let’s see how to model the APIs that can be used to affect the state change of an aggregate in both of these situations.

You’ll define two functions in a module named Snapshot, because the basic purpose of changing the state of an aggregate is to generate its current snapshot. The following listing introduces a few basic common types and defines the module Snapshot.

##### Listing 8.2. The API for state change of an aggregate and generated snapshot

```
import scalaz._
Import Scalaz._

object Common {                                                          #1
  type AggregateId = String
  type Error = String
}

import Common._

trait Aggregate {                                                        #2
  def id: AggregateId
}

trait Snapshot[A <: Aggregate] {
  def updateState(e: Event[_], initial: Map[String, A]): Map[String, A]  #3

  def snapshot(es: List[Event[_]]): String \/ Map[String, A] =
    es.reverse.foldLeft(Map.empty[String, A]) { (a, e) =>
      updateState(e, a) }.right                                          #4
}
```

In [listing 8.2](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex02), the function updateState is like a State monad.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn07" id="fn07b">7</a>]</sup> It takes an initial state and an Event and updates the state of the aggregate to the current snapshot. The processing within updateState depends on the aggregate and needs to be implemented as part of the aggregate-specific functionality. The function snapshot is generic; it’s just a fold over the list of events supplied and generates the current snapshot for all the participating aggregates.

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The next obvious step is to give you an idea of how updateState works for our use case, as shown in the following listing.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn08" id="fn08b">8</a>]</sup>

##### Listing 8.3. The state-changing API for the Account aggregate

```
object AccountSnapshot extends Snapshot[Account] {
  def updateState(e: Event[_], initial: Map[String, Account])= e match {
    case o @ Opened(no, name, odate, _) =>
      initial + (no -> Account(no, name, odate.get))                     #1

    case c @ Closed(no, cdate, _) =>
      initial + (no -> initial(no).copy(dateOfClosing =
        Some(cdate.getOrElse(today))))

    case d @ Debited(no, amount, _) =>
      val a = initial(no)
      initial + (no -> a.copy(balance =
        Balance(a.balance.amount - amount)))

    case r @ Credited(no, amount, _) =>
      val a = initial(no)
      initial + (no -> a.copy(balance =
        Balance(a.balance.amount + amount)))
  }
}
```

You’ve already accomplished quite a bit of the implementation of our event-sourced domain model. Let’s look at the summary of our achievements in this section with respect to our implementation:

-   Defined the algebra of events for processing accounts ([listing 8.1](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex01)).
-   Defined a state-changing API that can be used to change the state of an aggregate, depending on the event ([listing 8.2](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex02)). You’ll see how to use it when you implement command handlers.
-   Defined an API for snapshotting that uses the state-changing API in the preceding bullet point ([listing 8.3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex03)). This gives you an idea of how to use snapshotting to re-create a specific state of an application domain model. For example, you can pick up a list of events between two dates and ask your model to prepare a snapshot. Note you can do this only because you have the entire event stream that the system has ever processed.

In the next section, you’ll move on to the more interesting part: command processing.

### 8.4.2. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Commands as free monads over events

Every command has a handler to perform the actions that the command is supposed to do. In a successful execution, an event is published in the event log. So a command [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)executes actions, and you model a Command as a free monad over Event. Your command-processing loop will be modeled in the following way:

-   A command is a pure data type. Because it’s a monad, you can compose commands to form larger commands. This composite command that you build is still an abstraction that has no semantics or operation.
-   All effects of command processing take place in the interpreter of the free monad when you add semantics to the data type. You’ll use the algebra of events to find the appropriate command handler and execute actions and publish events.

If this sounds complicated, wait until you see the implementation. If you understood the explanation of free monads in [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05), you’ll be able to relate to the same concepts here as well. And along the way, you’ll discover the virtues of a purely functional event-sourced domain model implementation. The following listing provides the base module of your Command.

##### Listing 8.4. Command as a free monad

```
trait Commands[A] {
type Command[A] = Free[Event, A]                                         #1
}

trait AccountCommands extends Commands[Account] {
  import Event._
  import scala.language.implicitConversions

  private implicit def liftEvent[A](event: Event[A]): Command[A] =
    Free.liftF(event)                                                    #2

  def open(no: String, name: String,
    openingDate: Option[DateTime]): Command[Account] =
      Opened(no, name, openingDate, today)                               #3

  def close(no: String, closeDate: Option[DateTime]): Command[Account] =
    Closed(no, closeDate, today)

  def debit(no: String, amount: Amount): Command[Account] =
    Debited(no, amount, today)

  def credit(no: String, amount: Amount): Command[Account] =
    Credited(no, amount, today)
}
```

Intuitively, [listing 8.4](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex04) defines the commands such as open and close, and each maps to an Event type, which it will publish on successful execution when you allow for its interpretation. You also have an implicit function, liftEvent, that lifts the Event into the context of its free monad. This explains why each command becomes a free [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)monad of type Command\[Account\]. [Figure 8.8](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fig08) explains this transformation with a simple diagram.

##### Figure 8.8. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)You model a command as a pure data type, which is a free monad over the event it publishes.

![](https://drek4537l1klr.cloudfront.net/ghosh2/Figures/08fig08_alt.jpg)

By virtue of being a monad, you can compose commands algebraically to form larger ones:

```
val composite =
  for {
    a <- open("a-123", "John J", Some(today))
    _ <- credit(a.no, 10000)
    _ <- credit(a.no, 30000)
    d <- debit(a.no, 23000)
  } yield d
```

Here composite is still a pure data type. You’ll see in the next section how to peel off the layers of monads from a composite command, extract the events that hide under it, interpret the algebra of the events, and assign actions to each of them.

### 8.4.3. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Interpreters—hideouts for all the interesting stuff

As experts of free monads, I’m sure you know by now that the ideal place to process commands with side effects is the interpreter of the free monad. You build your commands through composition and finally submit to the interpreter for doing the actual stuff. Let’s have a simple interpreter that dispatches on the event that each of our commands publish and does all the necessary domain logic. The following listing provides one such interpreter.

##### Listing 8.5. Interpreter of the free monad

```
import scalaz._
Import Scalaz._
import scalaz.concurrent.Task
trait RepositoryBackedInterpreter {
  def step: Event ~> Task

  def apply[A](action: Free[Event, A]): Task[A]                          #1
    = action.foldMap(step)
}
```

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Let’s talk a bit about how this interpreter works. The implementation looks too succinct, and that’s because you haven’t yet provided the details of the domain logic processing. But this is the kernel of how the interpretation of the free monad takes place, and if you understand these four lines of code, you should be able to appreciate the details. Here are the steps that this interpreter uses to process a command, which, however, can be composite:<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn09" id="fn09b">9</a>]</sup>

-   apply is the entry point of the interpreter, which takes a free monad as its input. In this case, you can pass a command, which is Free\[Event, Account\]. Note that this free monad is a recursive data structure, so you can have layers of commands that need to be executed, one after the other (much like the composite example from the preceding section).
-   The basic function of the apply method is to recurse through the entire stack of commands and return a Task that you can execute. The free monad implementation of Scalaz has a function, foldMap, that _folds over_ the monad and maps the step function for each layer. The step function returns a Task that again is a monad.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn10" id="fn10b">10</a>]</sup> So all the Tasks that foldMap generates _flatten_ together to form the final Task that apply returns.<sup>[<a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08fn11" id="fn11b">11</a>]</sup>
-   The step function returns a natural transformation, which, as you saw in [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05), is a structure-preserving transformation. Here you build a Task from an Event. Finally, the interpreter returns a scalaz.concurrent.Task that you can use with one of its supported strategies for evaluation.

#### [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The interpreter that delivers account service

You’re now into the final parts of our implementation, where all the domain behaviors need to execute, command handlers need to fire, side effects need to run, and events need to be generated. That sounds like a mouthful, but the implementation is straightforward. Until now, we’ve organized our code as pure modules; even commands are (until now) pure data types. You’ll soon give semantics to these inert commands as part of our account-specific interpreter. The following listing gives an overview of structuring our interpreter. I won’t give the whole implementation here; as usual, you can find a completely runnable version as part of the online code repository for the book.

##### Listing 8.6. The domain logic for processing commands

```
object RepositoryBackedAccountInterpreter extends
  RepositoryBackedInterpreter {
  import AccountSnapshot._

  val eventLog = InMemoryEventStore.apply[String]                        #5

  import eventLog._

  val step: Event ~> Task = new (Event ~> Task) {
    override def apply[A](action: Event[A]): Task[A] =
      handleCommand(action)
  }

private def validateClose(no: String, cd: Option[DateTime]) = for {      #1
  l <- events(no)                                                        #2
  s <- snapshot(l)                                                       #3
  a <- closed(s(no))
  _ <- beforeOpeningDate(a, cd)                                          #6
} yield s

  // all other domain validation functions ..

private def handleCommand[A](e: Event[A]): Task[A] = e match {           #4

  case o @ Opened(no, name, odate, _) => Task {
    validateOpen(no).fold(
      err => throw new RuntimeException(err),
      _ => {
        val a = Account(no, name, odate.get)
        eventLog.put(no, o)
        a
      }
    )
}

  case c @ Closed(no, cdate, _) => Task {
    validateClose(no, cdate).fold(
      err => throw new RuntimeException(err),
      currentState => {
        eventLog.put(no, c)
        updateState(c, currentState)(no)
      }
    )
  }

// all other command handlers ..

}
```

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The three main parts of RepositoryBackedAccountInterpreter are as follows:

-   _Event log_—You define the event log here. In our use case, it’s a simple in-memory Map (you can have a look at the source code in the repository). But you have a generic interface, and you can have multiple implementations for the same. In [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)fact, you have an implementation in the online repository that stores JSON instead of the event objects.
-   _Domain validations_—An important aspect of command handlers is executing domain validations, and you define the validations as part of the interpreter. I’ve shown one validation function here just to give you an idea of how to use monadic effects to look up the aggregate from the event log ❶, prepare its snapshot ❷, and then invoke the validation functions on it ❸.
-   _Command handler_—All commands get executed as part of the interpretation of our event algebra. The apply method of RepositoryBackedInterpreter in [listing 8.5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex05) is the entry point of the command handler. It takes the whole command stack, recurses through the structure, and passes every event to the handle-Command function. Note the event that it passes is the one that the command used when it built up the free monad (❶ in [listing 8.4](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex04)). So you started with the command, built as a free monad over an event, composed a bunch of those to form composite commands, and now the command handler executes those commands by interpreting the events (❹ in [listing 8.6](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex06)). It uses the event algebra and does a pattern match. Then the validations are done, and the handler writes the event into the event log. Finally, the command handler uses the update-State API to update the state of the aggregate (Account) by applying the current event to it.

#### [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)A sample run

Let’s tie all the strings together and see what a sample run looks like. [Listing 8.7](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex07) has been manually formatted to fit these pages, but you’ll get a similar result when you run the code from the book’s online repository. This section has covered parts of the implementation to highlight the functional aspects of the way it works. You can get the complete runnable implementation from the online repository.

In the online repository, you’ll see a few more abstractions that make the implementation modular and reusable. You’ll see the implementation split into generic components that can be reused across any aggregate. And you’ll have some account-specific logic as well (for example, the specific commands and events that apply only to an account).

##### Listing 8.7. A sample run

```
scala> import frdomain.ch8.cqrs.service._
scala> import Scripts._
scala> import RepositoryBackedAccountInterpreter._

scala> RepositoryBackedAccountInterpreter(composite)                     #1
res0: scalaz.concurrent.Task[Account] = scalaz.concurrent.Task@2aaa2ff0

scala> res0.unsafePerformAsync
res1: Account = Account(a-123,debasish ghosh,2015-11-
     22T12:00:09.000+05:30,None,Balance(17000))                          #2

scala> for {
    | a <- credit("a-123", 1000)
    | b <- open("a-124", "john j", Some(org.joda.time.DateTime.now()))
    | c <- credit(b.no, 1200)
    | } yield c
res2: scalaz.Free[Event,Account] = Gosub(..)

scala> RepositoryBackedAccountInterpreter(res2)
res3: scalaz.concurrent.Task[Account] = scalaz.concurrent.Task@3bb6d65c

scala> res3.unsafePerformSync
res4: Account = Account(a-124,john j,2015-11-
    22T12:01:23.000+05:30,None,Balance(1200))                            #3

scala> eventLog.events("a-123")
res5: scalaz.\/[Error,List[Event[_]]] = \/-(List(Credited(a-123,1000,
 2015-11-22T12:00:09.000+05:30,<function1>), Debited(a-123,23000,
 2015-11-22T12:00:09.000+05:30,<function1>), …

scala> eventLog.events("a-124")
res6: scalaz.\/[Error,List[Event[_]]] = \/-(List(Credited(a-124,1200,
 2015-11-22T12:00:09.000+05:30,<function1>), Opened(a-124,
 john j,Some(2015-11-22T12:01:23.000+05:30),2015-11-
     22T12:00:09.000+05:30,<function1>)))                                #4

scala> import AccountSnapshot._
import AccountSnapshot._

scala> import scalaz._
import scalaz._

scala> import Scalaz._
import Scalaz._

scala> res5 |+| res6
res7: scalaz.\/[Error,List[Event[_]]] = \/-(List(Credited(
 a-123,1000,2015-11-22T12:00:09.000+05:30,<function1>),
 Debited(a-123,23000,2015-11-22T12:00:09.000+05:30,<function1>), …      #5

scala> res7 map snapshot
res8: scalaz.\/[Error,scalaz.\/[String,Map[String,Account]]] =
 \/-(\/-(Map(a-124 -> Account(a-124,john j,2015-11-
     22T12:01:23.000+05:30,None,Balance(1200)), a-123 -> Account(a-123,
 debasish ghosh,2015-11-22T12:00:09.000+05:30,None,
 Balance(18000)))))                                                     #6

scala> eventLog.allEvents
res9: scalaz.\/[Error,List[Event[_]]] = \/-(List(Credited(a-123,
 1000,2015-11-22T12:00:09.000+05:30,<function1>),
 Debited(a-123,23000,2015-11-22T12:00:09.000+05:30,<function1>), …

scala> res9 map snapshot
res10: scalaz.\/[Error,scalaz.\/[String,Map[String,Account]]] =
 \/-(\/-(Map(a-124 -> Account(a-124,john j,
 2015-11-22T12:01:23.000+05:30,None,Balance(1200)),
 a-123 -> Account(a-123,debasish ghosh,
 2015-11-22T12:00:09.000+05:30,None,Balance(18000)))))                  #7
#1 A sample composite command, which is a free monad being interpreted by your interpreter generating a Task
#2 Runs the task to create a snapshot of the account that the composite command created through some debits and credits
#3 Creates another composite command that creates another account, does some operations on it, and also on the account created in the preceding step
#4 Fetches the set of events for both the accounts from the event log
#5 Note the return type of events is a scalaz.\/ that’s a monoid. You can combine them with monoid append and get the total set of events.
#6 Now you can run snapshot on this set of events and get the latest snapshot for both the accounts.
#7 You can also fetch all events from the event log at once and run a snapshot on the full set. This is useful when you want to build your aggregate snapshot from scratch using the event stream from event log.
```

### 8.4.4. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Projections—the read side model

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)So far, you’ve seen how to process commands and publish events that get appended to an event log. But what about queries and reports that your model needs to serve? [Section 8.3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08lev1sec3) introduced read models designed specifically for this purpose. Read models are also called _projections_, which are essentially mappings from the write model (event log) into forms that are easier to serve as queries. For example, if you serve your queries from a relational database, the table structures will be suitably denormalized so that you can avoid joins and other expensive operations and render queries straightaway from that model of data.

Setting up a projection is nothing more than having an appropriate snapshotting function that reads a stream of events and updates the current model. This update needs to be done for the new events that get generated in the event log. You need to consider a few aspects of projection architecture before deciding on how you’d like to have your read models:

-   _Push or pull_—You can have the write side push the new events to update the read model. Alternatively, the read model can pull at specific intervals, checking whether any new events are there for consumption. Both models have advantages and disadvantages. The pull model may be wasteful if the rate of event generation is slow and intermittent; the read side may consume lots of cycles pulling from empty streams. The push model is efficient in this respect: Events get pushed only when they’re generated. But pull models have built-in back-pressure handling; the read side pulls, depending on the rate at which it can consume. Push models need an explicit back-pressure handling mechanism, as you saw with Akka Streams in [chapter 7](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-7/ch07).
-   _Restarting projections_—In some cases, you may need to change your read model schema, maybe because of a change in the event structure in the write model. In such a situation, you need to migrate to the updated version of the model with minimal impact on the query/reporting service. This is a tricky situation, and the exact strategy may depend on the size and volume of your read model data. One strategy is to start hydrating a new projection from the event log while still serving queries from the older one. After the new projection model catches up with all events from the write model, you switch to serving from the new one.

### 8.4.5. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The event store

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)The _event store_ is the core storage medium for all domain events that have been created in your system to date. You need to give lots of thought to selecting an appropriate store that meets the criteria of scalability and performance for your model. Fortunately, this storage needs much simpler semantics than for a relational database. This is because the only write operation that you want to do is an append. Many append-only stores are available that offer scalability, including Event Store ([https://geteventstore.com](https://geteventstore.com/)) or some of the NoSQL stores such as Redis ([http://redis.io](http://redis.io/)) or Cassandra ([http://cassandra.apache.org](http://cassandra.apache.org/)).

The nature of storage in an event store is simple. You need to store events indexed by aggregate ID. You should have at least the following operations from such a store:

-   Get a list of events for a specific aggregate.
-   Put an event for a specific aggregate into the store.
-   Get all events from the event store.

The following listing shows the general contract for such an event store.

##### Listing 8.8. A simple contract for an event store

```
import scalaz.\/
trait EventStore[K] {
  def get(key: K): List[Event[_]]
  def put(key: K, event: Event[_]): Error \/ Event[_]
  def events(key: K): Error \/ List[Event[_]]
  def allEvents: Error \/ List[Event[_]]
}
```

The online repository for this book contains a couple of implementations for an event store, which you can go through. The event store is one of the central artifacts that can make or break the reliability of your model. Our examples have used a simple implementation based on a thread-safe concurrent Map. But in reality, you need to choose implementations that guarantee reliability as well as high availability. In most production-ready implementations, writes are fsynced to a specific number of drives by using quorums before you declare the data to have been safely persisted. Some implementations also offer ACID semantics over transactions in event stores. You can have long-lived transactions and treat the whole as an atomic one—just as in an RDBMS.

### 8.4.6. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Distributed CQRS—a short note

Event sourcing and CQRS rely on event recording and replays for managing application state. Event stores typically need to be replicated across multiple locations, or nodes across a single location, or even processes across a single node. Also, you may have multiple event stores, each having different event logs collaborating together through replication to form a composite application state. All replication has to be asynchronous. Hence you need a strategy for resolving conflicting updates. If you have a domain model that needs to implement distributed domain services that rely [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)on multiple event stores collaborating toward consistency, be sure to pick up CQRS frameworks that offer such capabilities.

These frameworks replicate events across locations, using protocols that preserve the happened-before relationship (causality) between events. Typically, they use a vector-clock-based implementation to ensure tracking of causality. One such framework is Eventuate ([http://rbmhtechnology.github.io/eventuate](http://rbmhtechnology.github.io/eventuate)), which is based on Akka Persistence’s event-sourced actor implementation ([http://doc.akka.io/docs/akka/2.4.4/scala/persistence.html](http://doc.akka.io/docs/akka/2.4.4/scala/persistence.html)). Eventuate uses event-sourced actors for command handling and event-sourced views and writers for queries (projection model). Using event-sourced processors, you can build event-processing pipelines in Eventuate that can form the backbone of your distributed CQRS implementation.

This section is a reference for implementing distributed CQRS-based domain services. You won’t implement the details of the service. You can look at frameworks such as Eventuate if you need an implementation, but in most cases you may not need distribution of services. And this is true even if your model has multiple bounded contexts that are decoupled in space and time. Keep data stores local to your bounded context and ensure that services in each bounded context access data only from that store. Design each bounded context as an independently managed and deployable unit. This design technique has a popular name today: _microservices_. Chris Richardson has a nice collection of patterns that you can follow to design microservice-based architectures ([http://microservices.io/patterns/microservices.html](http://microservices.io/patterns/microservices.html)).

One of the most important consequences of this paradigm is that you have a clear separation between bounded contexts and don’t have to manage causality of events across contexts. It’s also not mandatory that you implement CQRS in all of the bounded contexts. Each context may have its own technique of data management. But because a decoupling between contexts occurs, you never have to deal with resolution of conflicts in data consistency across cross-context services. The net result is that within each bounded context where you decide to implement CQRS, you can follow the purely functional implementation that you saw in this chapter.

### 8.4.7. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Summary of the implementation

It has been a long story so far, with the complete implementation of the event-sourced version of persisting aggregates. You didn’t try a generic framework, but instead followed a specific use case to see the step-by-step evolution of the model. Let’s go through a summary of the overall implementation so that you get a complete picture of how to approach the problem for your specific use case:

1.  Define an algebra for the events. Use an algebraic data type and have specialized data constructors for each type of event.
2.  Define the command as a free monad over the event. This makes commands composable. You can build composite commands through a for-comprehension with individual commands. The resultant command is still just a data type without any associated semantics. Hence a command is a pure abstraction under our model of construction.
3.  [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Use the algebra of events in the interpreter to do command processing. This way, you have all effects within the interpreter, keeping the core abstraction of a command pure. On successful processing, commands publish events as part of the interpretation logic, which get appended to the event log.
4.  Define a suitable read model (also known as a projection), and use an appropriate strategy to hydrate the read model from the event stream on the write side.

## [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)8.5. Other models of persistence

CQRS and event sourcing offer a persistence model that provides a functional implementation. Events are nothing more than serialized functions, descriptions of something that has already happened. It’s no coincidence that many of the implementations of event sourcing give a functional interface of interaction. But the paradigm has quite a bit of complexity in architecture that you can’t ignore:

-   _Different paradigm_—It’s a completely different way to work with your data layer, miles away from what you’ve been doing so far with your RDBMS-based application. You can’t ignore the fact that it’s not familiar territory to most data architects.
-   _Operational complexity_—With separate write and read models, you need to manage more moving parts. This adds to the operational complexity of the architecture.
-   _Versioning_—With a long-lived domain model, you’re bound to have changes in model artifacts. This will lead to changes in event structures over time. You have to manage this with event versioning. Many approaches to this exist, and none of them is easy. If you plan to have an event-sourced data model for your application, plan for versioned events from day one.

In view of these issues, it’s always useful to keep in mind that you can use a relational model for your data in a functional way with your domain model. You no longer need to use an object relational framework that mandates mutability in your model. We’re seeing libraries that help you do so, using the benefits of immutable algebraic data types and functional combinators along with your domain APIs to interact with an underlying RDBMS. This section presents a brief overview of Slick, an open source, functional relational mapping library for Scala. But this isn’t a detailed description of how Slick works. For more details, see the Slick website ([http://slick.typesafe.com](http://slick.typesafe.com/)) or the excellent book _Essential Slick_ by Richard Dallaway and Jonathan Ferguson ([http://underscore.io/books/essential-slick/](http://underscore.io/books/essential-slick/)).

### 8.5.1. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Mapping aggregates as ADTs to the relational tables

Continuing with our example of the Account aggregate, [listing 8.9](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex09) defines a composite aggregate for a customer account and all of its balances across a period of time. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Note the Balance type contains a field, asOnDate, that specifies the date on which the balance amount is recorded.

##### Listing 8.9. Account and Balance aggregates

```
import java.sql.Timestamp

case class Account(id: Option[Long],
  no: String,
  name: String,
  address: String,
  dateOfOpening: Timestamp,
  dateOfClosing: Option[Timestamp]
)

case class Balance(id: Option[Long],
  account: Long,                                                         #1
  asOnDate: Timestamp,
  amount: BigDecimal
)
```

As you can see, this is a shallow aggregate design. Instead of storing a reference to an Account within Balance, you store an account ID. This is often considered a good practice when you’re dealing with a relational model underneath, because it keeps your domain model aggregate closer to the underlying relational structure. _Effective Aggregate Design_, an excellent series of articles by Vaughn Vernon, presents some of the best practices in designing aggregates (see [](https://vaughnvernon.com/?p=838)https://vaughnvernon.co/?p=838).

Using Slick, you can map your aggregate structure to the underlying relational mode. The following listing shows how to do this and how to define your table structure as an internal DSL in Scala.

##### Listing 8.10. The table definitions in Scala DSL using Slick

```
import Accounts._
import Balances._
class Accounts(tag: Tag) extends Table[Account](tag, "accounts") {       #1
  def id = column[Long]("id", O.PrimaryKey, O.AutoInc)                   #2
  def no = column[String]("no")
  def name = column[String]("name")
  def address = column[String]("address")
  def dateOfOpening = column[Timestamp]("date_of_opening")
  def dateOfClosing = column[Option[Timestamp]]("date_of_closing")

  def * = (id.?, no, name, address, dateOfOpening, dateOfClosing)
    <> (Account.tupled, Account.unapply)                                 #3
  def noIdx = index("idx_no", no, unique = true)
}
object Accounts {
  val accounts = TableQuery[Accounts]
}

class Balances(tag: Tag) extends Table[Balance](tag, "balances") {
  def id = column[Long]("id", O.PrimaryKey, O.AutoInc)
  def account = column[Long]("account")
  def asOnDate = column[Timestamp]("as_on_date")
  def amount = column[BigDecimal]("amount")
 
  def * = (id.?, account, asOnDate, amount)
    <> (Balance.tupled, Balance.unapply)
  def accountfk = foreignKey("ACCOUNT_FK", account, accounts)
    (_.id, onUpdate=ForeignKeyAction.Cascade,
      onDelete=ForeignKeyAction.Cascade)                                 #4
}
object Balances {
  val balances = TableQuery[Balances]
}
```

[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)This maps our ADTs Account and Balance with the underlying table structures by defining them as shapes that can be produced through data manipulation with the underlying database. Slick does this mapping through a well-engineered structure within its core. The net value that you gain is a seamless interoperability between the database structure and our immutable algebra of types. You’ll see this in action when we discuss how to manipulate data using the functional combinators of Slick.

### 8.5.2. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Manipulating data (functionally)

As a library in Scala, Slick offers functional combinators for manipulating data that look a lot like the Scala Collections API. This makes using the Slick APIs more comfortable for a Scala user. Consider the example in the following listing; you’d like to query from your database, using the schema defined in [listing 8.10](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex10), the collection of Balance records for a specific Account within a specified time interval.

##### Listing 8.11. Balance query using functional combinators of Slick

```
def getBalance(db: Database, accountNo: String, fromDate: Timestamp,
  toDate: Timestamp) = {                                                 #1
  val action = for {
    a <- accounts.filter(_.no === accountNo)
    b <- balances if a.id === b.account &&
                  b.asOnDate >= fromDate && b.asOnDate <= toDate
  } yield b                                                              #2
  db.run(action.result.asTry)                                            #3
}
```

As you can see, the core query API resembles the same model as the Scala collections. The query accounts.filter fetches a projection from the Accounts table and binds monadically with the query that works on the Balances table through the [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)[](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)for-comprehension. This is, in effect, a relational join implemented monadically. For more variants of joins and queries, refer to the documentation on Slick ([http://slick.typesafe.com](http://slick.typesafe.com/)). The query returns a Future, which, as you know by now is one of the core substrates that make your model reactive. Instead of waiting on the Future, you can compose with other APIs that return Futures and build larger, nonblocking APIs. But Slick offers more-reactive APIs by allowing you to stream your database query results directly into an Akka Streams–based pipeline. You’ll take a brief look at this capability next.

### 8.5.3. [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)Reactive fetch that pipelines to Akka Streams

Consider a use case for fetching a large volume of data from your database and processing it as part of your domain logic. In order to be reactive with bounded latency and guaranteed response time from the server, you can use the power of streams. And reactive frameworks such as Slick allow you to directly publish your database query result as a source into your Akka Streams pipeline ([http://doc.akka.io/docs/akka/2.4.4/scala/stream/index.html](http://doc.akka.io/docs/akka/2.4.4/scala/stream/index.html)).

Here’s a simple query that fetches the sum of all balances grouped by accounts within a specified time period. This is bound to be a huge result set for a nontrivial database of any financial company that serves retail customers. The following listing shows the query implemented using Slick.

##### Listing 8.12. Balance query that generates a stream of result

```
type BalanceRecord = (String, Timestamp, Timestamp, Option[BigDecimal])
def getTotalBalanceByAccount(db: Database, fromDate: Timestamp,
  toDate: Timestamp): DatabasePublisher[BalanceRecord] = {               #1
  val action = (for {
    a <- accounts
    b <- balances if a.id === b.account && b.asOnDate >= fromDate
                     && b.asOnDate <= toDate
  } yield (a.no, b)).groupBy(_._1).map { case (no, bs) =>
    (no, fromDate, toDate, bs.map(_._2.amount).sum)
  }                                                                      #2
  db.stream(action.result)                                               #3
}
```

This listing has a couple of important takeaways for accessing your database in a reactive way:

-   _Back-end query power_—Slick offers lots of combinators to process data at the server level and can generate optimized SQL for them. In [listing 8.12](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08ex12), use of groupBy is a similar example. By using groupBy, you can do the grouping at the server level only (much as you would with SQL) so that you save on expensive data transport to do the formatting on the client side.
-   _Reactive streams integration_—The query returns a DatabasePublisher, which you can hook on straightaway to an Akka Streams flow graph as a Source; for example, val accountSource: Source\[BalanceRecord\] = Source(getTotalBalanceByAccount(..)).

In summary, you can use a relational database with an appropriate functional library to model your database as a reactive persistence layer.

## [](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/)8.6. Summary

This chapter was an introduction to persisting domain models in an underlying database. Gone are the days when we assumed that we’d have a relational database for storing our domain-model elements. New paradigms have emerged, and we’ve learned to better align our functional domain model with underlying storage that also values immutability of data. We discussed event sourcing and CQRS that eschew the concept of in-place updates of data and instead store your domain model as an immutable stream of events. The main takeaways of this chapter are as follows:

-   _CRUD isn’t the only model of persistence_—We discussed the rationale for using events as the source of ground truth and introduced event sourcing as one technique to store domain events. This makes our model more auditable and traceable and leads to a huge increase in the perceived business value of the underlying storage. It’s not only storage of relational data; it’s now storage of domain events as well.
-   _Implementing an event-sourced domain model functionally_—We demonstrated how to implement an event-sourced model using functional techniques. Lots of other implementations are available today. But the approach we took goes well with the principles of functional programming. Free monads give you a pure implementation of commands and event algebras, whereas interpreters deal with side effects.
-   _FRM, not ORM_—If you decide to use the CRUD model for persistence (and there are valid reasons to do so), use a functional relational mapping framework such as Slick. We discussed in brief how to do so with a purely functional interface to the relational database underneath.

___

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn01b" id="ch08fn01">1</a></sup>  Or claims to manage.

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn02b" id="ch08fn02">2</a></sup>  ACID stands for Atomic, Consistent, Isolated, and Durable.

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn05b" id="ch08fn05">5</a></sup>  For more information on the event-sourcing pattern, see [https://msdn.microsoft.com/en-us/library/dn589792.aspx](https://msdn.microsoft.com/en-us/library/dn589792.aspx).

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn06b" id="ch08fn06">6</a></sup>  If you need a refresher, feel free to jump back to [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05) and read the sections on free monads.

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn07b" id="ch08fn07">7</a></sup>  I discussed the `State` monad extensively in [section 4.2.3](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-4/ch04lev2sec5) in [chapter 4](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-4/ch04).

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn08b" id="ch08fn08">8</a></sup>  We use `Account` as an aggregate. For brevity, the `Account` definition isn’t included here. The online repository for [chapter 8](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/ch08) code provides details. The exact definition of `Account` isn’t important for understanding how states change.

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn09b" id="ch08fn09">9</a></sup>  Understanding exactly how the interpreter works requires knowledge of the way free monads are implemented in Scalaz. This is covered in [chapter 5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05), [section 5.5.5](https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-5/ch05lev2sec12).

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn10b" id="ch08fn10">10</a></sup>  `Task` is a monad; see [https://github.com/scalaz/scalaz](https://github.com/scalaz/scalaz) for `Task` source code.

<sup><a href="https://livebook.manning.com/book/functional-and-reactive-domain-modeling/chapter-8/fn11b" id="ch08fn11">11</a></sup>  Remember, a monad offers a `flatMap`, which is a `map`, followed by `flatten`. Here all `Task`s that are generated `flatMap` to form the final `Task`.