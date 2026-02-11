-   The challenges of consistency in a distributed application
-   Synchronous and asynchronous communication
-   Using sagas to develop business logic across multiple services
-   API composition and CQRS for microservice queries

Many monolithic applications rely on transactions to guarantee consistency and isolation when changing application state. Obtaining these properties is straightforward: an application typically interacts with a single database, with strong consistency guarantees, using frameworks that provide support for starting, committing, or rolling back transactional operations. Each logical transaction might involve several distinct entities; for example, placing an order will update transactions, reserve stock positions, and charge fees.

You’re not so lucky in a microservice application. As you learned earlier, each independent service is responsible for a specific capability. Data ownership is decentralized, ensuring a single owner for each “source of truth.” This level of decoupling helps you gain autonomy, but you sacrifice some of the safety you were previously afforded, making consistency an application-level problem. Decentralized data ownership also makes retrieving data more complex. Queries that previously used database-level joins now require calls to multiple services. This is acceptable for some use cases but painful for large data sets.

Availability also impacts your application design. Interactions between services might fail, causing business processes to halt, leaving your system in an inconsistent state.

In this chapter, you’ll learn how to use _sagas_ to coordinate complex transactions across multiple services and explore best practices for efficiently querying data. Along the way, we’ll examine different types of event-based architectures, such as event sourcing, and their applicability to microservice applications.

## 5.1 Consistent transactions in distributed applications

Imagine you’re a customer at SimpleBank and you want to sell some stock. If you recall chapter 2, this involves several operations ([figure 5.1](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.1)):

1.  You create an order.
2.  The application validates and reserves the stock position.
3.  The application charges you a fee.
4.  The application places the order to the market.

From your perspective as a customer, this operation appears to be atomic: charging a fee, reserving stock, and creating an order happen at the same time, and you can’t sell stock that you don’t have or sell a stock you do have more than once.

In many monolithic applications,<sup><a id="c05-footnoteref-1" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-1">1</a></sup>  those requirements are easy to meet: you can wrap your database operations in an ACID transaction and rest easy in the knowledge that errors will cause an invalid state to be rolled back.

##### 

[Figure 5.1](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.1) Placing a sell order

![c05_01.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_01.png)

##### 

[Figure 5.2](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.2) Failure occurs when charging a fee in your cross-service order placement process

![c05_02.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_02.png)

By contrast, in your microservice application, each of the actions in [figure 5.1](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.1) is performed by a distinct service responsible for a subset of application state. Decentralized data ownership helps ensure services are independent and loosely coupled, but it forces you to build application-level mechanisms to maintain overall data consistency.

Let’s say an orders service is responsible for coordinating the process of selling a stock. It calls account transactions to reserve stock and then the fees service to charge the customer. But that transaction fails. (See [figure 5.2](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.2).)

At this stage, your system is in an inconsistent state: stock is reserved, an order is created, but you haven’t charged the customer. You can’t leave it like this — so the implementation of orders needs to initiate corrective action, instructing the account transactions service to compensate and remove the stock reservation. This might look simple, but it becomes increasingly complex when many services are involved, transactions are long-running, or an action triggers further interleaved downstream transactions.

### 5.1.1 Why can’t you use distributed transactions?

Faced with this problem, your first impulse might be to design a system that achieves transactional guarantees across multiple services. A common approach is to use the two-phase commit(2PC)protocol.<sup><a id="c05-footnoteref-2" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-2">2</a></sup>  In this approach, you use a transaction manager to split operations across multiple resources into two phases: prepare and commit ([figure 5.3](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.3)).

##### 

[Figure 5.3](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.3) The prepare and commit phases of a 2PC protocol

![c05_03.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_03.png)

This sounds great — like what you’re used to. Unfortunately, this approach is flawed. First, 2PC implies synchronicity of communication between the transaction manager and resources. If a resource is unavailable, the transaction can’t be committed and must roll back. This in turn increases the volume of retries and decreases the availability of the overall system. To support asynchronous service interactions, you would need to support 2PC with services _and_ the messaging layer between them, limiting your technical choices.

##### NOTE

In a microservice application, availability is the product of all microservices involved in processing a given action. Because no service is 100% reliable, involving more services lessens overall reliability, increasing the probability of failure. We’ll explore this in detail in the next chapter.

Handing off significant orchestration responsibility to a transaction manager also violates one of the core principles of microservices: service autonomy. At worst, you’d end up with dumb services representing CRUD operations against data, with transaction managers wholly encapsulating the interesting behavior of your system.

Finally, a distributed transaction places a lock on the resources under transaction to ensure isolation. This makes it inappropriate for long-running operations, as it increases the risk of contention and deadlock. What should you do instead?

## 5.2 Event-based communication

Earlier in this book, we discussed using events emitted by services as a communication mechanism. Asynchronous events aid in decoupling services from each other and increase overall system availability, but they also encourage service authors to think in terms of _eventual_ _consistency_. In an eventually consistent system, you design complex outcomes to result from several independent local transactions over time, which leads you to explicitly design underlying resources to represent tentative states. From the perspective of Eric Brewer’s CAP theorem,<sup><a id="c05-footnoteref-3" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-3">3</a></sup>  this design approach prioritizes the availability of underlying data.

##### 

[Figure 5.4](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.4) The synchronous process of placing a sell order

![c05_04.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_04.png)

To illustrate the difference between a synchronous and an asynchronous approach, let’s return to the sell order example. In a synchronous approach ([figure 5.4](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.4)), the orders service orchestrates the behavior of other services, invoking a sequence of steps until the order is placed to the market. If any steps fail, the orders service is responsible for initiating rollback action with other services, such as reversing the charge.

In this approach, the orders service takes on substantial responsibility:

-   It knows which services it needs to call, as well as their order.
-   It needs to know what to do in case any downstream service produces an error or can’t proceed due to business rules.

Although this type of interaction is easy to reason through — as the call graph is logical and sequential — this level of responsibility tightly couples the orders service to other services, limiting its independence and increasing the difficulty of making future changes.

### 5.2.1 Events and choreography

You can redesign this scenario to use events ([figure 5.5](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.5)). Each service subscribes to events that interest it to know when it must perform some work:

1.  When the user issues a sell request via the UI, the application publishes an `OrderRequested`event.
2.  The orders service picks up this event, processes it, and publishes back to the event queue an `OrderCreated`event.
3.  Both the transaction and fees services then pick up this event. Each one of them performs its work and publishes back events to notify about the completion.
4.  The market service in turn is waiting for a pair of events notifying it of the charging of fees and the reservation of stocks. When both arrive, it knows it can place the order against the stock exchange. Once that’s finished, the market service publishes a final event back to the queue.

Events allow you to take an optimistic approach to availability. For example, if the fees service were down, the orders service would still be able to create orders. When the fees service came back online, it could continue processing a backlog of events. You can extend this to rollback: if the fees service fails to charge because of insufficient funds, it could emit a `ChargeFailed` event, which other services would then consume to cancel order placement.

This interaction is _choreographed_: each service reacts to events, acting independently without knowledge of the overall outcome of the process. These services are like dancers: they know the steps and what to do in each section of a musical piece, and they react accordingly without you needing to explicitly invoke or command them. In turn, this design decouples services from each other, increasing their independence and making it easier to deploy changes independently.

##### 

[Figure 5.5](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.5) Services consuming and emitting events for order placement

![c05_05.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_05.png)

##### Events and the monolith

An event-oriented approach to service communication shines when migrating a monolithic application to microservices. By emitting events from the monolith, you consume them in microservices that you’re developing in parallel. This way, you can build new features without tightly coupling your monolith to your new services.

Think about it: you emit an event, and that’s the only change you need to implement on the monolith to make an external system work alongside the current one, lowering risk and enabling safer experimentation on new services.

## 5.3 Sagas

The choreographed approach is a basic example of the _saga_ pattern. A saga is a coordinated series of local transactions; a previous step triggers each step in the saga.

The concept itself significantly predates the microservice approach. Hector Garcia-Molina and Kenneth Salem originally described sagas in a 1987 paper<sup><a id="c05-footnoteref-4" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-4">4</a></sup>  as an approach toward long-lived transactions in database systems. As with distributed transactions, locking in long-lived transactions reduces availability — a saga solves this as a sequence of interleaved, individual transactions.

As each local transaction is atomic — but not the saga as a whole — a developer must write their code to ensure that the system ultimately reaches a consistent state, even if individual transactions fail. Pat Helland’s famous paper, “Life Beyond Distributed Transactions,”<sup><a id="c05-footnoteref-5" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-5">5</a></sup>  suggests that you can think of this as uncertainty — an interaction across multiple services may not have a guaranteed outcome. In a distributed transaction, you manage uncertainty using locks on data; without transactions, you manage uncertainty through semantically appropriate workflows that confirm, cancel, or compensate for actions as they occur.

Before we talk about sell orders and services, let’s look at a simple real-world saga: purchasing a cup of coffee.<sup><a id="c05-footnoteref-6" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-6">6</a></sup>  Typically, this might involve four steps: ordering, payment, preparation, and delivery ([figure 5.6](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.6)). In the normal outcome, the customer pays for and receives the coffee they ordered.

##### 

[Figure 5.6](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.6) The process of purchasing a cup of coffee

![c05_06.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_06.png)

##### 

[Figure 5.7](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.7) Purchasing a cup of coffee with compensating actions

![c05_07.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_07.png)

This can go wrong! The coffee shop machine might break; the barista might make a cappuccino, but I wanted a flat white; they might give my coffee to the wrong customer; and so on. If one of these events occurs, the barista will naturally compensate: they might make my coffee again or refund my payment ([figure 5.7](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.7)). In most cases, I’ll eventually get my coffee.

You use compensating actions in sagas to undo previous operations and return your system to a more consistent state. The system isn’t guaranteed to be returned to the _original_ state; the appropriate actions depend on business semantics. This design approach makes writing business logic more complex — because you need to consider a wide range of potential scenarios — but is a great tool for building reliable interactions between distributed services.

### 5.3.1 Choreographed sagas

Let’s return to the earlier example — sell orders — to better understand how you can apply the saga pattern to your microservices. The actions in this saga are choreographed: each action, T<sub>X</sub>, is performed in response to another, but without an overall conductor or orchestrator. You can break this task into five subtasks:

-   T1 — Create the order.
-   T2 — Reserve the stock position, which the account transaction service implements.
-   T3 — Calculate and charge the fee, which the fees service implements.
-   T4 — Place the purchase order to the market, which the market service implements.
-   T5 — Update the status of the order to be placed.

[Figure 5.8](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.8) illustrates the optimistic — most likely — path of this interaction.

##### 

[Figure 5.8](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.8) A saga for processing a sell order

![c05_08.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_08.png)

Let’s explain the five steps of this process:

1.  The orders service performs T1 and emits an OrderCreated event.
2.  The fees, account transactions, and market services consume this event.
3.  The fees and account transactions services perform appropriate actions (T2 and T3) and emit events, and the market service consumes them.
4.  When the prerequisites for the order are met, the market service places the order (T4) to the market and emits an OrderPlaced event.
5.  Lastly, the orders service consumes that event and updates the status of the order (T5).

Each of these tasks might fail — in which case, your application should roll back to a sane, consistent state. Each of your tasks has a compensating action:

-   C1 — Cancel the order that the customer created.
-   C2 — Reverse the reservation of stock positions.
-   C3 — Revert the fee charge, refunding the customer.
-   C4 — Cancel the order placed to market.
-   C5 — Reverse the state of the order.

What triggers these actions? You guessed it — events! For example, imagine that placing the order to market fails. The market service will cancel the order by emitting an event — OrderFailed — that each other service involved in this saga consumes. When receiving the event, each service will act appropriately: the orders service will cancel the customer’s order; the transaction service will cancel the stock reservation; and the fees service will reverse the fee charged, executing actions C1, C2, and C3, respectively. This is shown in [figure 5.9](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.9).

##### 

[Figure 5.9](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.9) The market service emits a failure event is to initiate a rollback process across multiple services.

![c05_09.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_09.png)

This form of rollback is intended to make the system _semantically_, not mathematically consistent. Your system on rollback of an operation may not be able to return to the exact same initial state. Imagine one of the tasks executed on calculating the fees was sending out an email. You can’t unsend an email, so you’d instead send another one acknowledging the error and saying the amount that the fees service had charged was deposited back to the account.

Every action involved in a process might have one or more appropriate compensating actions. This approach adds to system complexity — both in anticipating scenarios and in coding for them and testing them — especially because the more services involved in an interaction, the greater the possible intricacy of rolling back.

Anticipating failure scenarios is a crucial part of building services that reflect real-world circumstance, rather than operating in isolation. When designing microservices, you need to take compensation into account to ensure that the wider application is resilient.

#### Advantages and drawbacks

The choreographed style of interaction is helpful because participating services don’t need to explicitly know about each other, which ensures they’re loosely coupled. In turn, this increases the autonomy of each service. Unfortunately, it’s not perfect.

No single piece of your code knows how to execute a sell order. This can make validation challenging, spreading those rules across multiple distinct services. It also increases the complexity of state management: each service needs to reflect distinct states in the processing of an order. For example, the orders service must track whether an order has been created, placed, canceled, rejected, and so on. This additional complexity increases the difficulty of reasoning about your system.

Choreography also introduces cyclic dependencies between services: the orders service emits events that the market service consumes, but, in turn, it also consumes events that the market service emits. These types of dependencies can lead to release time coupling between services.

Generally, when opting for an asynchronous communication style, you must invest in monitoring and tracing to be able to follow the execution flow of your system. In case of an error, or if you need to debug a distributed system, the monitoring and tracing capabilities act as a flight recorder. You should have all that happens stored there so you can later investigate every single event to make sense of what happened in a multitude of systems. This capability is crucial for choreographed interactions.

##### NOTE

Chapters 11 and 12 will explore how to achieve observability through logging, tracing, and monitoring in microservice applications.

A choreographed approach makes it difficult to know how far along a process is. Likewise, the order of rollback might be important; this isn’t guaranteed by choreography, which has looser time guarantees than an orchestrated or synchronous approach. For simple, near-instant workflows, knowing where you’re at is often irrelevant, but many business processes aren’t instant — they might take multiple days and involve disparate systems, people, and organizations.

### 5.3.2 Orchestrated sagas

Instead of choreography, you can use _orchestration_ to implement sagas. In an orchestrated saga, a service takes on the role of orchestrator (or coordinator): a process that executes and tracks the outcome of a saga across multiple services. An orchestrator might be an independent service — recall the verb-oriented services from chapter 4 — or a capability of an existing service.

The sole responsibility of the orchestrator is to manage the execution of the saga. It may interact with participants in the saga via asynchronous events or request/response messages. Most importantly, it should track the state of execution for each stage in the process; this is sometimes called the _saga log_.

Let’s make the orders service a saga coordinator. [Figure 5.10](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.10) illustrates the happy path where a customer places an order successfully.

##### 

[Figure 5.10](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.10) An orchestrated saga for placing an order

![c05_10.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_10.png)

You’ll quickly see the key difference between this and the choreographed example from [figure 5.8](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.8): the orders service tracks the execution of each substep in the process of placing an order. It’s useful to think of the coordinator as a state machine: a series of states and transitions between those states. Each response from a collaborator triggers a state change, moving the orchestrator toward the saga outcome.

As you know, a saga won’t always be successful. In an orchestrated saga, the coordinator is responsible for initiating appropriate reconciliation actions to return the entities affected by the failed transaction to a valid, consistent state.

Like you did earlier, imagine the market service can’t place the order to market. The orchestrating service will initiate compensating actions:

1.  It’ll issue a request to the account transaction service to reverse the lock placed on the holdings to be sold.
2.  It’ll issue a request to cancel the fee that was charged to the customer.
3.  It may change the state of the order to reflect the outcome of the saga — for example, to rejected or failed. This depends on the business logic (and whether failed orders should be shown to the customer or retried).

In turn, the orchestrator also could track the outcome of actions 1 and 2. [Figure 5.11](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.11) illustrates this failure scenario.

##### 

[Figure 5.11](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.11) In this unsuccessful saga, a failure by the market service results in the orchestrator triggering compensating actions.

![c05_11.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_11.png)

##### TIP

Don’t forget that compensating actions might not all happen instantaneously or at the same time. For example, if the fee was charged to a customer’s debit card, it might take a week for their bank to reverse the charge.

But if the desired actions you want to happen can fail, the compensating actions — or the orchestrator itself — also could fail. You should design compensating actions to be safe to retry without unintentional side effects (for example, double refunds). At worst, repeated failure during rollback might require manual intervention. Thorough error monitoring should catch these scenarios.

#### Advantages and drawbacks

Centralizing the saga’s sequencing logic in a single service makes it significantly easier to reason about the outcome and progress of that saga, as well as change the sequencing in one place. In turn, this can simplify individual services, reducing the complexity of states they need to manage, because that logic moves to the coordinator.

This approach does run the risk of moving too much logic to the coordinator. At worst, this makes the other services anemic wrappers for data storage, rather than autonomous and independently responsible business capabilities.

Many microservice practitioners advocate peer-to-peer choreography over orchestration, as they see this approach to reflect the “smart endpoints, dumb pipes” aim of microservice architecture, in contrast to the heavy workflow tools (such as WS-BPEL) people often used in enterprise SOA. But orchestrated approaches are becoming increasingly popular in the community, especially for building long-running interactions, as seen by the popularity of projects like Netflix Conductor and AWS Step Workflows.

### 5.3.3 Interwoven sagas

Unlike ACID transactions, sagas aren’t isolated. The result of each local transaction is immediately visible to other transactions affecting that entity. This visibility means that a given entity might get simultaneously involved in multiple, concurrent sagas. As such, you need to design your business logic to expect and handle intermediate states. The complexity of the interleaving required primarily depends on the nature of the underlying business logic.

For now, imagine that a customer placed an order by accident and wanted to cancel it. If they issued their request before the order was placed to market, the order placement saga would still be in progress, and this new instruction would potentially need to interrupt it ([figure 5.12](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.12)).

Three common strategies for handling interwoven sagas are available: short-circuiting, locking, and interruption.

##### 

[Figure 5.12](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.12) Steps in sagas may be interwoven

![c05_12.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_12.png)

#### Short-circuiting

You could prevent the new saga from being initiated while the order is still within another saga. For example, the customer couldn't cancel the order until after the market service attempted to place it to the market. This isn’t great for a user but is probably the easiest strategy!

#### Locking

You could use locks to control access to an entity. Different sagas that want to change the state of the entity would wait to obtain the lock. You’ve already seen an example of this in action: you place a reservation — or lock — on a stock balance to ensure that a customer can’t sell a holding twice if it’s involved in an active order.

This can lead to deadlocks if multiple sagas block each other trying to access the lock, requiring you to implement deadlock monitoring and timeouts to make sure the system doesn’t grind to a halt.

#### Interruption

Lastly, you could choose to interrupt the actions taking place. For example, you could update the order status to “failed.” When receiving a message to send an order to market, the market gateway could revalidate the latest order status to ensure the order was still valid to send, and in this case it would see a “failed” status. This approach increases the complexity of business logic but avoids the risk of deadlocks.

### 5.3.4 Consistency patterns

Although sagas rely heavily on compensating actions, they’re not the only approach you might use to achieve consistency in service interactions. So far, we’ve encountered two patterns for dealing with failure: compensating actions (refund my coffee payment) and retries (try to make the coffee again). [Table 5.1](https://livebook.manning.com/book/microservices-in-action/chapter-5/table5.1) outlines other strategies.

##### Table 5.1 Consistency strategies in microservice applications

| **#** | **Name** | **Strategy** |
| --- | --- | --- |
| 1 | Compensating action | Perform an action that undoes prior action(s) |
| 2 | Retry | Retry until success or timeout |
| 3 | Ignore | Do nothing in the event of errors |
| 4 | Restart | Reset to the original state and start again |
| 5 | Tentative operation | Perform a tentative operation and confirm (or cancel) later |

The use of these strategies will depend on the business semantics of your service interaction. For example, when processing a large data set, it might make sense to ignore individual failures (applying strategy #3), because the cost of processing the overall data set is large. When interacting with a warehouse — for example, to fulfill orders — it’d be reasonable to place a tentative hold (strategy #5) on a stock item in a customer’s basket to reduce the possibility of overselling.

### 5.3.5 Event sourcing

So far, we’ve assumed that entity state and events are distinct: the former is stored in an appropriate transactional store, whereas the latter are published independently ([figure 5.13](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.13)).

An alternative to this approach is the _event sourcing_ pattern: rather than publishing events _about_ entity state, you represent state entirely as a sequence of events that have happened to an object. To get the state of an entity at a specific time, you aggregate events before that date. For example, imagine your orders service:

-   In the traditional persistence approaches we’ve assumed so far, a database would store the latest state of the order.
-   In event sourcing, you’d store the events that happened to change the state of the order. You could materialize the current state of the order by replaying those events.

##### 

[Figure 5.13](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.13) A service storing state in a data store and publishing events, in two distinct actions

![c05_13.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_13.png)

##### 

[Figure 5.14](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.14) An order, stored as a sequence of events

![c05_14.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_14.png)

[Figure 5.14](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.14) illustrates the event sourcing approach for tracking an order’s history.

This architecture solves a common problem in enterprise applications: understanding how you reached your current state. It removes the division between state and events; you don’t need to stick events on top of your business logic, because your business logic inherently generates and manipulates events. On the other hand, it makes complex queries more difficult: you’d need to materialize views to perform joins or filter by field values, as your event storage format would only support retrieving entities by their primary key.

Event sourcing isn’t a requirement for a microservice application, but using events to store application state can be a particularly elegant tool, especially for applications involving complex sagas where tracking the history of state transitions is vital. If you’re interested in learning more about event sourcing, Nick Chamberlain’s awesome-ddd list ([https://github.com/heynickc/awesome-ddd](https://github.com/heynickc/awesome-ddd)) has a great collection of resources and further reading.

## 5.4 Queries in a distributed world

Decentralized data ownership also makes retrieving data more challenging, as it’s no longer possible to aggregate related data at, or close to, the database level — for example, through joins. Presenting data from disparate services is often necessary at the UI layer of an application.

For example, imagine you’re building an administrative UI that shows a list of customers, together with their current open orders. In a SQL database, you’d join these two tables in a single query, returning one dataset. In a microservice application, this _composition_ would typically take place at the API level: a service or an API gateway could perform this ([figure 5.15](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.15)). C_orrelation IDs_ — roughly analogous to foreign keys in a relational database — identify relationships between data that each service owns; for example, each order would record the associated customer ID.

The two-step approach in [figure 5.15](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.15) works well for single entities or small datasets but will scale poorly for bulk requests. If the first query returns N customers, then the second query will be performed N times, which could quickly get out of hand. If we were querying a SQL database, this would be trivial to solve with a join, but because our data is spread across multiple data stores, an easy solution like using a join isn’t possible.

We could improve this query by introducing bulk request endpoints and paging, as in [listing 5.1](https://livebook.manning.com/book/microservices-in-action/chapter-5/listing5.1). Rather than getting every customer, you’d get the first page; rather than retrieving customer orders one-by-one, you could retrieve them with a list of IDs. You should note, though, that if each customer had thousands of orders, having to page those as well would add substantial overhead.

##### Listing 5.1 Different endpoints for data retrieval

```
123
```

API composition is simple and intuitive, and for many use cases, such as individual aggregates or small enumerables, the performance of this approach will be acceptable. For others, such as the following, performance will be inefficient and far from ideal:

-   _Queries that return and join substantial data, such as reporting_ — “I want all customer orders from the last year.”
-   _Queries that aggregate or perform analytics across multiple services_ — “I want to know the average order value of emerging market stocks purchased by customers over 35.”
-   _Queries that aren’t optimally supported by the service’s own database_ — For example, complex search patterns are often difficult to optimize in relational databases.

##### 

[Figure 5.15](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.15) Data composition at the API level

![c05_15.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_15.png)

Lastly, API composition is impacted by availability. Composition requires synchronous calls to underlying services, so the total availability of a query path is the product of the availability of all services involved in that path. For example, if the two services and the API gateway in [figure 5.15](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.15) each have an availability of 99%, their availability when called together would be 99%^3: 97.02%. Over the next three sections, we’ll discuss how you also can use events to build efficient queries in microservice applications.

##### NOTE

We’ll discuss service availability and reliability, and techniques for maximizing those properties in the following chapter.

### 5.4.1 Storing copies of data

You can elect to have services store or cache data that they receive from other services via events. For example, in [figure 5.16](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.16), when the fees service receives an `OrderCreated` message, it might elect to store additional detail about the order, beyond the correlation ID. This service can now handle queries like “What was the value of this order?” without needing to retrieve that data with an additional call to the orders service.

This technique can be quite useful but risky:

-   Maintaining multiple copies of data increases overall application and service complexity (and possibly, overall storage cost).
-   Breaking schema changes in events is extremely tricky to manage, as services become increasingly coupled to event content.
-   Cache invalidation is notoriously hard.<sup><a id="c05-footnoteref-7" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-7">7</a></sup> 

##### 

[Figure 5.16](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.16) You can use events to share, and therefore replicate, state across multiple services

![c05_16.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_16.png)

By maintaining canonical data in multiple locations — updated via asynchronous events, which could be delayed, or fail, or be delivered multiple times — you have to cope with eventual consistency and the chance that the copies of data you retrieve have become stale.

Whether it’s fine for data to be stale sometimes is down to the business semantics of the particular feature. But it’s a hard tradeoff. The CAP theorem<sup><a id="c05-footnoteref-8" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-8">8</a></sup>  says that you can’t have things both ways: you need to choose between availability — returning a successful result, without a guarantee that data is fresh — and consistency — returning the most recent state, or an error.

Guaranteeing consistency tends to result in increased coordination between systems — such as distributed locks — which hampers transaction speed. In contrast, a system that maximizes availability ultimately relies on compensating actions and retries — a lot like sagas. From an architectural perspective, availability is usually easier to achieve and, because of the reduced coordination cost, more amenable to building scalable applications.

##### Prioritizing availability

Building systems that prioritize availability might require you to avoid the instinctual, consistency-oriented solution to a problem. Even systems that seem like they should prioritize consistency often make availability tradeoffs to maximize successful use.

A great example is an automated teller machine (ATM) — prioritizing availability increases bank revenue. If an ATM can’t connect to the bank backend, or the wider ATM network, it’ll still allow withdrawals, but cap them, ensuring risk of overdraft is limited. If a withdrawal does place a customer in overdraft, the bank can recoup that with a fee.

A recent article from Eric Brewer — [http://mng.bz/HGA3](http://mng.bz/HGA3) — has a great overview of this scenario.

### 5.4.2 Separating queries and commands

You can generalize the previous approach — using events to build views — further. In many systems, queries are substantially different from writes: whereas writes affect singular, highly normalized entities, queries often retrieve denormalized data from a range of sources. Some query patterns might benefit from completely different data stores than writes; for example, you might use PostgreSQL as a persistent transactional store but Elasticsearch for indexing search queries. The command-query responsibility segregation pattern (CQRS) is a general model for managing these scenarios by explicitly separating reads (queries) from writes (commands) within your system.<sup><a id="c05-footnoteref-9" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-9">9</a></sup> 

##### NOTE

We won’t go into specific technical detail about implementing CQRS, but you can explore frameworks in many languages, such as Commanded (Elixir), CQRS.net (.NET), Lagom (Java and Scala), and Broadway (PHP).

#### CQRS architecture

Let’s sketch out this architecture. In [figure 5.17](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.17), you can see that CQRS partitions commands and queries:

-   The _command_ side of an application performs updates to a system — creates, updates and deletes. Commands emit events, either in-band or to a distinct event bus, such as RabbitMQ or Kafka.
-   Event handlers consume events to build appropriate _query_ or _read_ models.
-   A separate data store may support each side of the system.

You can apply this pattern both within services and across your whole application — using events to build dedicated query services that own and maintain complex views of application data. For example, imagine you wanted to aggregate order fees across your entire customer base, potentially slicing them by multiple attributes (for example, type of order, asset categories, payment method). This wouldn’t be possible at a service level, because neither the fees, orders, nor customers service has all the data needed to filter those attributes.

Instead, as [figure 5.18](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.18) illustrates, you could build a query service, CustomerOrders, to construct appropriate views. A query service is a good way to handle views that don’t clearly belong to any other services, ensuring a reasonable separation of concerns.

##### 

[Figure 5.17](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.17) CQRS partitions a service into command and query sides, each accessing separate data stores.

![c05_17.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_17.png)

##### 

[Figure 5.18](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.18) Query services can construct complex views from events that multiple services emit.

![c05_18.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_18.png)

##### TIP

You don’t need to use only CQRS within your application. Using different query styles in different scenarios can help achieve a good balance of complexity, implementation speed, and customer value.

So far, this all sounds great! In a microservices application, CQRS offers two key benefits:

-   You can optimize the query model for specific queries to improve their performance and remove the need for cross-service joins.
-   It aids in separation of concerns, both within services and at an application level.

But it’s not without drawbacks. Let’s explore those now.

### 5.4.3 CQRS challenges

Like the data caching example, CQRS requires you to consider eventual consistency because of _replication lag_: inherently, the command state of a service will be updated before the query state. Because events update query models, someone querying that data might receive an out of date view. This might be a frustrating user experience ([figure 5.19](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.19)). Imagine you update the value of an order, but on clicking Confirm, you see the details of the original order! Web UIs that use a POST/redirect/GET<sup><a id="c05-footnoteref-10" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-10">10</a></sup>  pattern will often suffer from this problem.

##### 

[Figure 5.19](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.19) Lag in updating a query view leads to inconsistent results when making a request.

![c05_19.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_19.png)

In some systems, this might not matter. For example, delayed updates are common for activity feeds<sup><a id="c05-footnoteref-11" href="https://livebook.manning.com/book/microservices-in-action/chapter-5/c05-footnote-11">11</a></sup>  — if I post an update on Twitter, it doesn’t matter if my followers don’t all receive it at thesame time. And in fact, attempting to achieve greater consistency can lead to substantial scalability challenges that might not be worth it.

In other systems, it’ll be important to ensure you don’t query invalid state. You can apply three strategies ([figure 5.20](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.20)) in these scenarios: optimistic updates, polling, or publish-subscribe.

#### Optimistic updates

You could update the UI optimistically, based on the expected result of a command. If the command fails, you can roll back the UI state. For example, imagine you like a post on Instagram. The app will show a red heart before the Instagram backend saves that change. If that save fails, Instagram will roll back the optimistic UI change, and you’ll have to like it again for it to show a red heart.

This approach relies on having — or being able to derive — all the information you need to update the UI from the command input, so it works best when working with simple entities.

##### 

[Figure 5.20](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.20) Three strategies for dealing with query-side replication lag in CQRS

![c05_20.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_20.png)

#### Polling

The UI could poll the query API until an expected change has occurred. When initiating a command, the client would set a version, such as a timestamp. For subsequent queries, the client would continue to poll until the version number was equal or greater to the version number specified, indicating that the query model had been updated to reflect the new state.

#### Publish-subscribe

Instead of polling for changes, a UI could subscribe to events on a query model — for example, over a web socket channel. In this case, the UI would only update when the read model published an “updated” event.

As you can see, it’s challenging to reason through CQRS, and it requires a different mindset from what you’d have when dealing with normal CRUD APIs. But it can be useful in a microservice application. Done right, CQRS helps to ensure performance and availability in queries, even as you distribute data and responsibility across multiple distinct services and data stores.

### 5.4.4 Analytics and reporting

You can generalize the CQRS technique to other use cases, such as analytics and reporting. You can transform a stream of microservice events and store them in a data warehouse, such as Amazon Redshift or Google BigQuery ([figure 5.21](https://livebook.manning.com/book/microservices-in-action/chapter-5/figure5.21)). A transformation stage may involve mapping events to the semantics and data model of the target warehouse or combining events with data from other microservices. If you don’t yet know how you want to treat or query events, you can store them in commodity storage, such as Amazon S3, for later querying or reprocessing with big data tools such as Apache Spark or Presto.

##### 

[Figure 5.21](https://livebook.manning.com/book/microservices-in-action/chapter-5#figureanchor5.21) You can use microservice events to populate data warehouses or other analytic stores.

![c05_21.png](https://drek4537l1klr.cloudfront.net/bruce/Figures/c05_21.png)

## 5.5 Further reading

We’ve covered a lot of ground in this chapter, but some topics, like sagas, event sourcing, and CQRS, can each fill entire books. In case you’re interested in knowing more about those topics, we recommend the following books:

-   _Reactive Application Development_, by Duncan K. DeVore, Sean Walsh, and Brian Hanafee, [https://www.manning.com/books/reactive-application-development](https://www.manning.com/books/reactive-application-development) (ISBN 9781617292460)
-   _Microservices Patterns_, by Chris Richardson, [https://www.manning.com/books/microservices-patterns](https://www.manning.com/books/microservices-patterns) (ISBN 9781617294549)
-   _Event Streams in Action_, by Alexander Dean, [https://www.manning.com/books/event-streams-in-action](https://www.manning.com/books/event-streams-in-action) (ISBN 9781617292347)

## Summary

-   ACID properties are difficult to achieve in interactions across multiple services; microservices require different approaches to achieve consistency.
-   Coordination approaches, such as two-phase commit, introduce locking and don’t scale well.
-   An event-based architecture decouples independent components and provides a foundation for scalable business logic and queries in a microservice application.
-   Biasing towards availability, rather than consistency, tends to lead to a more scalable architecture.
-   Sagas are global actions composed from message-driven, independent local transactions. They achieve consistency by using compensating actions to roll back incorrect state.
-   Anticipating failure scenarios is a crucial element of building services that reflect real-world circumstance, rather than operating in isolation.
-   You typically implement queries across microservices by composing results from multiple APIs.
-   Efficient complex queries should use the CQRS pattern to materialize read models, especially where those query patterns require alternative data stores.