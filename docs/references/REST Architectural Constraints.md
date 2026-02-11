REST stands for **Re**presentational **S**tate **T**ransfer, a term coined by [Roy Fielding](https://en.wikipedia.org/wiki/Roy_Fielding) in 2000. It is an **architecture style** for designing loosely coupled applications over the network, that is often used in the development of web services.

REST does not enforce any rule regarding how it should be implemented at the lower level, it just puts high-level design guidelines and leaves us to think of our own implementation.

In my last employment, I designed RESTful APIs for a major telecom company for two good years. In this post, I will be sharing my thoughts apart from standard design practices. You may not agree with me on a few points, and that’s perfectly OK. I will be happy to discuss anything with you with an open mind.

Let’s start with standard design-specific stuff to clarify what ‘Roy Fielding’ wants us to build. Then we will discuss my thoughts, which will be more towards finer points while you design your RESTful APIs.

## Architectural Constraints

REST defines **6 architectural constraints** that make any web service – a truly RESTful API.

1.  Uniform interface
2.  Client-server
3.  Stateless
4.  Cacheable
5.  Layered system
6.  Code on demand (optional)

## 1\. Uniform interface

As the constraint name itself applies, you MUST decide APIs interface for resources inside the system which are exposed to API consumers and follow religiously. A resource in the system should have only one logical URI, and that should provide a way to fetch related or additional data. It’s always better to **synonymize a resource with a web page**.

Any single resource should not be too large and contain each and everything in its representation. Whenever relevant, a resource should contain **links (HATEOAS) pointing to relative URIs** to fetch related information.

Also, the resource representations across the system should follow specific guidelines such as naming conventions, link formats, or data format (XML or/and JSON).

All resources should be accessible through a common approach such as HTTP GET and similarly modified using a consistent approach.

> Once a developer becomes familiar with one of your APIs, he should be able to follow a similar approach for other APIs.

## 2\. Client-server

This constraint essentially means that client applications and server applications MUST be able to evolve separately without any dependency on each other. A client should know only resource URIs, and that’s all. Today, this is standard practice in web development, so nothing fancy is required from your side. Keep it simple.

> Servers and clients can be independently developed or replaced, as long as the interface between them is not altered.

## 3\. Stateless

Roy fielding got inspiration from HTTP, so it reflected in this constraint. Make all client-server interactions stateless. The server will not store anything about the latest HTTP request the client made. It will treat every request as new. No session, no history.

If the client application needs to be a stateful application for the end-user, where the user logs in once and does other authorized operations after that, then each request from the client should contain all the information necessary to service the request – including authentication and authorization details.

> No client context shall be stored on the server between requests. The client is responsible for managing the state of the application.

## 4\. Cacheable

In today’s world, the caching of data and responses is of utmost importance wherever they are applicable/possible. The webpage you are reading here is also a cached version of the HTML page. Caching brings performance improvement for the client side and better scope for scalability for a server because the load has been reduced.

In REST, caching shall be applied to resources when applicable, and then these resources MUST declare themselves cacheable. Caching can be implemented on the server or client side.

> Well-managed caching partially or completely eliminates some client-server interactions, further improving scalability and performance.

## 5\. Layered system

REST allows you to use a layered system architecture where you deploy the APIs on server A, and store data on server B and authenticate requests in Server C, for example. A client cannot ordinarily tell whether it is connected directly to the end server or an intermediary along the way.

## 6\. Code on demand (optional)

Well, this constraint is optional. Most of the time, you will be sending the static representations of resources in the form of XML or JSON. But when you need to, you are free to `return executable code` to support a part of your application, e.g., clients may call your API to get a UI widget rendering code. It is permitted.

> All the above constraints help you build a truly RESTful API, and you should follow them. Still, at times, you may find yourself violating one or two constraints. Do not worry; you are still making a RESTful API – but not “truly RESTful.”

Notice that all the above constraints are most closely related to WWW (the web). Using RESTful APIs, you can do the same thing with your web services that you do to web pages.

Happy Learning !!