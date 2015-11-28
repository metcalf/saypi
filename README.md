# saypi (say-pee-eye)

Cowsay API to demonstrate several Golang design [patterns] we've used
successfully at Stripe.

## API Docs

We haven't found a consistent pattern for documenting service
APIs. Sharing client code helps but doesn't fulfill all the needs of
good documentation. For now we rely on
[manually maintained markdown files](api.md) such as the one in this
repository.

# Patterns

## Repositories

In our monolithic Ruby codebase, we access data via an
ActiveRecord-inspired library. Models are accessible to any part of
the application and any part of the code can perform essentially
arbitrary actions against the database (e.g. updating a particular
field, running custom commands). The ORM provides a lot of power out
of the box and abstracts away a ton of thinking about the interface
with your persistence layer. I'd argue that this is a mistake --
persistence is one of the most challenging parts of developing a
consistent and scalable application and shouldn't be treated as a
generic problem.

In saypi, all access to the database is wrapped by
[`repository` types](https://github.com/metcalf/saypi/blob/master/say/repository.go).
The *only* way to communicate with the database is through the narrow
interface defined on each `repository`. You'll never see a `Save`
method allowing you to update arbitrary data on a model. This makes
data access patterns more predictable, testable and refactorable. For
example:
* You can't accidentally write code in an unrelated module that
  triggers a full table scan.
* You can easily stub out the database in unrelated unit tests.
* You can refactor data access patterns in one place, with a clearly
  testable interface. For example, I was able to add read replicas
  with minimal code churn.

## Handler interface

We agree with
[Square's assessment that full-featured web frameworks are unncessary in Go](https://corner.squareup.com/2014/05/evaluating-go-frameworks.html). We
found that the specific choice of mux and handler interface doesn't
matter nearly as much as agreeing on a consistent pattern. For
example, it's important that all of our applications provide the same
request IDs in their log lines and can use the same package to record
consistent metrics.

We use [the context package](http://blog.golang.org/context) to pass
request-scoped values and our handlers use the signature `func(ctx
context.Context, rw http.ResponseWriter, req *http.Request)`.
[Goji](https://github.com/goji/goji) makes it easy to mix-and-match
our handlers and middleware with code that assumes an `http.Handler`.

## Error handling

Our first Go applications sufferred from two major error handling
problems.

First, unhandled errors would bubble up the stack and panic in the
controller. We would report the stacktrace to Sentry but it was
difficult to debug exactly where these errors originated (e.g. which query
caused that uniqueness violation in the database?). We like the
approach from [Juju's errors package](https://github.com/juju/errors)
of wrapping errors to include information such as the original
stacktrace. For unhandled errors we can log enough context to make
debugging possible and for well-handled errors we can log additional
context for operators while returning a sane response to the client.

The second problem was returning these sane and useful responses to
the client. Early on, I introduced a (harmless) server-side request
forgery vulnerability from an errant `err.Error()` return. I switched
to returning minimal information with error responses, making my
services harder to use. We've since settled on a pattern where any
error returned to the user must advertise a machine-readable error
code and human-readable message by implementing a `UserError`
interface. Human-readable messages can be detailed enough to make
debugging easy while machine-readable codes provide a complete
itemization of errors that the client should be prepared to
handle.

## Initialization and the `main` method

Get out of your `main` method as quickly as possible! Command-line
arguments and environment variables offer an untyped interface that
makes testing difficult and error-prone.

The `main` method should do little more than read configuration from
the environment. Saypi pushes the limit of what a `main` method should
do by binding to a port and setting up graceful
shutdown. Initialization such as connecting to the database should be
[handled by a separate `App` type](https://github.com/metcalf/saypi/blob/master/app/app.go)
that takes typed configuration from `main` and implements
`http.Handler`. The `App` is easy to instantiate in tests without
mucking with the environment and string-based configuration. It makes
it easy to write functional tests that exercise initialization paths
without deeply coupling packages in your app to each other.

## Missing pieces

We've learned a lot working Go into new services over the past year
but there are still a few places where we don't quite feel like we've
landed on the right patterns.
* Configuration: We've used a mix of command-line flags, environment
  variables and JSON files to configure our applications. Across
  Stripe, we like environment variables as a default but some services
  have complex configuration that's easier to represent in a
  file. Passing secrets to the application introduces further
  wrinkles.
* Healthchecking: Deep healthchecks risk falsely marking every
  instance of a service as down rather than entering a degraded state
  while very shallow healthchecks can mask problems on a single
  instance. This isn't a Go specific problem, but we'll likely have to
  solve it in Go as our primary language for new services.
* Testing: We tried suite-based interfaces like Gocheck and Ginkgo but
  felt like we were fighting the language. We much prefer the stdlib
  testing interfaces but haven't figured out how much to cede to an assertion
  library like testify.assert. For now, our rule is that anything goes as long
  as the tests are of the form `func Test*(t *testing.T)`.

Want to [help us solve these problems and much more](https://stripe.com/jobs/positions/engineer/)?

# TODOs and Qs
* Tests for invalid params to say controller
* Use API client to write sane tests
* Error codes and general error handling (including custom errors from
  middleware)
* Support rendering conversations as text instead of JSON
* Request IDs in log lines
* App should implement http.Handler

## Boring TODO
* Package descriptions
* Dependency management (vendor experiment?)

## Maybe TODO
* Use 201 created with Location header to force generating internal
  URLs? Return next/prev URLs for pagination in the Location header
  like Greenhouse?
* Generate public URLs for a conversation (maybe have auth package
  support returning a public version of any url?)
* What about user errors implementing a "type" that clients can unmarshal?
* frontend interface, Go as a static fileserver, JS tests running against stub
* Object creation limits
* Write an example of refactoring list queries to use a read-replica

## Maybe include these patterns?
* API clients across multiple languages
* Testing (pretty minimal... just use stdlib)
* DI?
* Dependency management?
