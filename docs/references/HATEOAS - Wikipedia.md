From Wikipedia, the free encyclopedia

**Hypermedia as the engine of application state** (**HATEOAS**) is a constraint of the [REST software architectural style](https://en.wikipedia.org/wiki/Representational_state_transfer "Representational state transfer") that distinguishes it from other network [architectural styles](https://en.wikipedia.org/wiki/Software_architecture#Architectural_styles_and_patterns "Software architecture").<sup id="cite_ref-1"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-1"><span>[</span>1<span>]</span></a></sup>

With HATEOAS, a client interacts with a network application whose application servers provide information dynamically through [hypermedia](https://en.wikipedia.org/wiki/Hypermedia "Hypermedia"). A REST client needs little to no prior knowledge about how to interact with an application or server beyond a generic understanding of hypermedia.

By contrast, clients and servers in [Common Object Request Broker Architecture](https://en.wikipedia.org/wiki/Common_Object_Request_Broker_Architecture "Common Object Request Broker Architecture") (CORBA) interact through a fixed [interface](https://en.wikipedia.org/wiki/Interface_(computing) "Interface (computing)") shared through documentation or an [interface description language](https://en.wikipedia.org/wiki/Interface_description_language "Interface description language") (IDL).

The restrictions imposed by HATEOAS decouple client and server. This enables server functionality to evolve independently.

The term was coined in 2000 by [Roy Fielding](https://en.wikipedia.org/wiki/Roy_Fielding "Roy Fielding") in his doctoral dissertation.<sup id="cite_ref-Fielding-Ch5_2-0"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-Fielding-Ch5-2"><span>[</span>2<span>]</span></a></sup>

A user-agent makes an HTTP request to a REST API through an entry point [URL](https://en.wikipedia.org/wiki/Uniform_Resource_Locator "Uniform Resource Locator"). All subsequent requests the user-agent may make are discovered inside the response to each request. The [media types](https://en.wikipedia.org/wiki/Media_type "Media type") used for these representations, and the [link relations](https://en.wikipedia.org/wiki/Link_relation "Link relation") they may contain, are part of the API. The client transitions through application states by selecting from the links within a representation or by manipulating the representation in other ways afforded by its media type. In this way, RESTful interaction is driven by hypermedia, rather than out-of-band information.<sup id="cite_ref-untangled2008_3-0"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-untangled2008-3"><span>[</span>3<span>]</span></a></sup>

For example, this GET request fetches an account resource, requesting details in a JSON representation:<sup id="cite_ref-cookbook_4-0"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-cookbook-4"><span>[</span>4<span>]</span></a></sup>

```
GET /accounts/12345 HTTP/1.1
Host: bank.example.com

```

The response is:

```
HTTP/1.1 200 OK

{
    "account": {
        "account_number": 12345,
        "balance": {
            "currency": "usd",
            "value": 100.00
        },
        "links": {
            "deposits": "/accounts/12345/deposits",
            "withdrawals": "/accounts/12345/withdrawals",
            "transfers": "/accounts/12345/transfers",
            "close-requests": "/accounts/12345/close-requests"
        }
    }
}

```

The response contains these possible follow-up links: POST a deposit, withdrawal, transfer, or close request (to close the account).

As an example, later, after the account has been overdrawn, there is a different set of available links, because the account is overdrawn.

```
HTTP/1.1 200 OK

{
    "account": {
        "account_number": 12345,
        "balance": {
            "currency": "usd",
            "value": -25.00
        },
        "links": {
            "deposits": "/accounts/12345/deposits"
        }
    }
}

```

Now only one link is available: to deposit more money (by POSTing to deposits). In its current _state_, the other links are not available. Hence the term _Engine of Application State_. What actions are possible varies as the state of the resource varies.

A client does not need to understand every media type and communication mechanism offered by the server. The ability to understand new media types may be acquired at run-time through "[code-on-demand](https://en.wikipedia.org/wiki/Code_on_demand "Code on demand")" provided to the client by the server.<sup id="cite_ref-Fielding-Ch5_2-1"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-Fielding-Ch5-2"><span>[</span>2<span>]</span></a></sup>

The HATEOAS constraint is an essential part of the "uniform interface" feature of REST, as defined in [Roy Fielding](https://en.wikipedia.org/wiki/Roy_Fielding "Roy Fielding")'s doctoral dissertation.<sup id="cite_ref-Fielding-Ch5_2-2"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-Fielding-Ch5-2"><span>[</span>2<span>]</span></a></sup> Fielding has further described the concept on his blog.<sup id="cite_ref-untangled2008_3-1"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-untangled2008-3"><span>[</span>3<span>]</span></a></sup>

The purpose of some of the strictness of this and other REST constraints, Fielding explains, is "software design on the scale of decades: every detail is intended to promote software longevity and independent evolution. Many of the constraints are directly opposed to short-term efficiency. Unfortunately, people are fairly good at short-term design, and usually awful at long-term design".<sup id="cite_ref-untangled2008_3-2"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-untangled2008-3"><span>[</span>3<span>]</span></a></sup>

-   [HTML](https://en.wikipedia.org/wiki/HTML "HTML") itself is hypermedia, with the `<form>...</form>` element in control of HTTP requests to links.<sup id="cite_ref-untangled2008_3-3"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-untangled2008-3"><span>[</span>3<span>]</span></a></sup><sup id="cite_ref-htmx_5-0"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-htmx-5"><span>[</span>5<span>]</span></a></sup> [Htmx](https://en.wikipedia.org/wiki/Htmx "Htmx") introduces extensions to HTML to allow elements other than `<form>...</form>` and `<a>...</a>` to control requests.
-   [HAL](https://en.wikipedia.org/wiki/Hypertext_Application_Language "Hypertext Application Language"), hypermedia built on top of JSON or [XML](https://en.wikipedia.org/wiki/XML "XML"). Defines links, but not actions (HTTP requests).
-   [JSON-LD](https://en.wikipedia.org/wiki/JSON-LD "JSON-LD"), standard for hyperlinks in JSON. Does not address actions.
-   [Siren](https://en.wikipedia.org/w/index.php?title=Siren_(specification)&action=edit&redlink=1 "Siren (specification) (page does not exist)"), hypermedia built on top of JSON. Defines links and actions.<sup id="cite_ref-6"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-6"><span>[</span>6<span>]</span></a></sup>
-   [Collection+JSON](https://en.wikipedia.org/w/index.php?title=Collection%2BJSON&action=edit&redlink=1 "Collection+JSON (page does not exist)"), hypermedia built on top of JSON. Defines links and actions.<sup id="cite_ref-7"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-7"><span>[</span>7<span>]</span></a></sup>
-   [JSON:API](https://en.wikipedia.org/w/index.php?title=JSON:API&action=edit&redlink=1 "JSON:API (page does not exist)"), defines links and actions.<sup id="cite_ref-8"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-8"><span>[</span>8<span>]</span></a></sup>
-   [Hydra](https://en.wikipedia.org/w/index.php?title=Hydra_(specification)&action=edit&redlink=1 "Hydra (specification) (page does not exist)"). Builds on top of JSON-LD to add definition of actions.<sup id="cite_ref-9"><a href="https://en.wikipedia.org/wiki/HATEOAS#cite_note-9"><span>[</span>9<span>]</span></a></sup>

-   [htmx](https://en.wikipedia.org/wiki/Htmx "Htmx")
-   [Hypertext Application Language](https://en.wikipedia.org/wiki/Hypertext_Application_Language "Hypertext Application Language")
-   [Universal Description Discovery and Integration](https://en.wikipedia.org/wiki/Universal_Description_Discovery_and_Integration "Universal Description Discovery and Integration") is the equivalent for the [Web Services Description Language](https://en.wikipedia.org/wiki/Web_Services_Description_Language "Web Services Description Language")

1.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-1 "Jump up")** M. Kelly (21 April 2024). ["Internet Draft : JSON Hypertext Application Language"](https://datatracker.ietf.org/doc/html/draft-kelly-json-hal-11). _datatracker.ietf.org_.
2.  ^ [Jump up to: <sup><i><b>a</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-Fielding-Ch5_2-0) [<sup><i><b>b</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-Fielding-Ch5_2-1) [<sup><i><b>c</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-Fielding-Ch5_2-2) Fielding, Roy Thomas (2000). ["Rizwan.Ali 03014821766 State Transfer (REST)"](https://www.ics.uci.edu/~fielding/pubs/dissertation/rest_arch_style.htm#sec_5_1_5). [_Architectural Styles and the Design of Network-based Software Architectures_](https://www.ics.uci.edu/~fielding/pubs/dissertation/top.htm) (PhD). [University of California, Irvine](https://en.wikipedia.org/wiki/University_of_California,_Irvine "University of California, Irvine"). p. 82. [ISBN](https://en.wikipedia.org/wiki/ISBN_(identifier) "ISBN (identifier)") [0599871180](https://en.wikipedia.org/wiki/Special:BookSources/0599871180 "Special:BookSources/0599871180").
3.  ^ [Jump up to: <sup><i><b>a</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-untangled2008_3-0) [<sup><i><b>b</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-untangled2008_3-1) [<sup><i><b>c</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-untangled2008_3-2) [<sup><i><b>d</b></i></sup>](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-untangled2008_3-3) [Fielding, Roy T.](https://en.wikipedia.org/wiki/Roy_Fielding "Roy Fielding") (20 Oct 2008). ["REST APIs must be hypertext-driven"](https://roy.gbiv.com/untangled/2008/rest-apis-must-be-hypertext-driven). Retrieved 20 May 2010.
4.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-cookbook_4-0 "Jump up")** Thijssen, Joshua (2016-10-29). ["What is HATEOAS and why is it important for my REST API?"](http://restcookbook.com/Basics/hateoas/). _REST CookBook_. Retrieved 2020-02-05.
5.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-htmx_5-0 "Jump up")** Gross, Carson. ["HATEOAS"](https://htmx.org/essays/hateoas/). _htmx.org_. This, despite the fact that neither XML nor JSON was a natural hypermedia in the same manner as HTML.
6.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-6 "Jump up")** [Siren: a hypermedia specification for representing entities](https://github.com/kevinswiber/siren) on [GitHub](https://en.wikipedia.org/wiki/GitHub "GitHub")
7.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-7 "Jump up")** ["Collection+JSON - Hypermedia Type"](http://amundsen.com/media-types/collection/). Retrieved 2021-10-25.
8.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-8 "Jump up")** ["JSON API: Latest Specification"](https://jsonapi.org/format). Retrieved 2021-10-25.
9.  **[^](https://en.wikipedia.org/wiki/HATEOAS#cite_ref-9 "Jump up")** ["Hydra: Hypermedia-Driven Web APIs"](http://www.markus-lanthaler.com/hydra). Retrieved 2021-10-27.