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

In our early Go applications we struggled to return useful errors to
our users and log the right details for debugging. We tried various
patterns that left us returning unhelpful errors, overly "helpful"
errors that exposed us to SSRF vulnerabilities or, at best, poorly
structured error data.

Errors may start deep in the application, far from the request
handler. In our Ruby applications we would throw a `UserError` that
bubbles all the way up the stack into error handling middleware.  This
deeply couples the entire application to a particular transport and
assumptions about user permissions. In Go, we prefer to propogate
descriptive types, following the patterns in the standard library.

In this snippet, the repository translates a known database error into
a package-specific type. For unknown errors it uses
[Juju's errors package](https://github.com/juju/errors) to trace the
location we first saw the error for logging to the operator.

```go
var errCursorNotFound = errors.New("Invalid cursor")
...
func (r *repository) listUserMoods(...) ([]Mood, bool, error) {
...
	if err == sql.ErrNoRows {
		return nil, false, errCursorNotFound
	} else if err != nil {
		return nil, false, errors.Trace(err)
	}
...
}
```

Only when the error reaches the request handler (or very near it) do
we determine how the error will translate into a serialized response
to the user. If the controller recognizes the error, it will translate
it into a concrete type that represents the error in a structured
way. If the controller does not recognize the error it does not panic;
it explicitly returns an error to the client. In either case, a helper
serializes the error into JSON and responds over the wire.

```go
func (c *Controller) ListMoods(ctx context.Context, w http.ResponseWriter, r *http.Request) {
...
	if err == errCursorNotFound {
		respond.UserError(ctx, w, http.StatusBadRequest, usererrors.InvalidParams{{
			Params:  []string{cursorParam},
			Message: "must refer to an existing object",
		}})
		return
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}
...
}
```

Every error we return to the client is
[represented by a concrete type](https://godoc.org/github.com/metcalf/saypi/usererrors)
and each type corresponds to a unique string code. Each error also
generates a human-readable message for easier debugging on the wire.
The library can serialize and deserialize these types to JSON.

```go
// InvalidParamsEntry represents a single error for InvalidParams
type InvalidParamsEntry struct {
	Params  []string `json:"params"`
	Message string   `json:"message"`
}

// InvalidParams represents a list of parameter validation
// errors. Each element in the list contains an explanation of the
// error and a list of the parameters that failed.
type InvalidParams []InvalidParamsEntry

// Code returns "invalid_params"
func (e InvalidParams) Code() string { return "invalid_params" }

// Error returns a joined representation of parameter messages.
// When possible, the underlying data should be used instead to
// separate errors by parameter.
func (e InvalidParams) Error() string {
	...
}
```

```json
{
  "code": "invalid_params",
  "error": "Parameter `starting_after` must refer to an existing object.",
  "data": [
    {
      "params": ["starting_after"],
      "message": "must refer to an existing object",
    }
  ]
}
```

Applications can register custom error types in addition to common
types provided by the library. Clients get access to structured, typed
details on the error rather than having to parse arbitrary string
messages and untyped metadata.

```go
// This makes an HTTP request to the application
err := cli.SetMood(&test.Mood)

switch usererr := err.(type) {
case nil:
	log.Println("all good!")
case usererrors.InvalidParams:
	for _, param := range usererr {
		log.Printf("%s: %s", param.Params, param.Message)
	}
case usererrors.NotFound:
	log.Printf("You must be dreaming. There is no such %s.", usererr.Resource)
default:
	log.Printf("I have no idea what to do with a %s.", err)
}
```

For a somewhat more complex example, consider returning additional
information from the repository layer and translating it into a
user-visible message. In this case, if deletion fails due to a
uniqueness violation, we return an error listing the conflicting IDs.

```go
type conflictErr struct{ IDs []string }
func (e conflictErr) Error() string { ... }

func (r *repository) DeleteMood(userID, name string) error {
	if isBuiltin(name) {
		return errBuiltinMood
	}

	queryArgs := struct{ UserID, Name string }{userID, name}
	if err := doDelete(r.deleteMood, queryArgs); err != nil {
		if dbErr, ok := errors.Cause(err).(*pq.Error); !ok || dbErr.Code != dbErrFKViolation {
			return err
		}

		var lineIDs []string
		if err := r.findMoodLines.Select(&lineIDs, queryArgs); err != nil {
			return errors.Trace(err)
		}

		return conflictErr{lineIDs}
	}

	return nil
}
```

Suppose, for example, we don't want to expose those IDs to the same
clients who can delete moods. The request handler can translate the
error from the repository into a sanitized message to the client that
only contains a count of the conflicting IDs.

```go
func (c *Controller) DeleteMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	err := c.repo.DeleteMood(userID, name)
	if conflict, ok := err.(conflictErr); ok {
		respond.UserError(ctx, w, http.StatusBadRequest, usererrors.ActionNotAllowed{
			Action: fmt.Sprintf("delete a mood associated with %d conversation lines", len(conflict.IDs)),
		})
	}
	...
}
```

For a more privileged client, we might return a completely different
error type that includes the actual IDs. In the client, we can type
assert to retrieve the original error and, for example, display the
message to the user. Though our error has limited structure this
time, it has enough that the client could use it to customize
the message to the user.

```go
// ActionNotAllowed describes an action that is not permitted.
type ActionNotAllowed struct {
	Action string `json:"action"`
}

// Code returns "action_not_allowed"
func (e ActionNotAllowed) Code() string { return "action_not_allowed" }

// Error returns a string describing the disallowed action
func (e ActionNotAllowed) Error() string {
	return fmt.Sprintf("You may not %s.", e.Action)
}
```

```json
{
  "code": "action_not_allowed",
  "error": "You may not delete a mood associated with 3 conversation lines.",
  "data": {
    "action": "delete a mood associated with 3 conversation lines",
  }
}
```

```go
err := cli.DeleteMood("cross")
if action, ok := err.(usererrors.ActionNotAllowed); ok {
    log.Printf("Seriously? You think you can just %s?", action.Action)
}
```

This pattern means that:
* Low-level modules are not tightly coupled to the transport and user
  permissions.
* Selective tracing allows operators to debug errors that originate
  deep in the stack.
* Error responses contain information for human debuggers as well
  as structured information for programmatic clients.
* Golang clients can easily reify the original error type and
  manipulate structured data.
* We don't panic.

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
  testing interfaces but haven't figured out how much to cede to an
  assertion library like testify.assert. For now, our rule is that
  anything goes as long as the tests are of the form `func Test*(t
  *testing.T)`.

Want to [help us solve these problems and much more](https://stripe.com/jobs/positions/engineer/)?

# TODOs and Qs

## Boring TODO
* Package descriptions
* Dependency management (vendor experiment?)
* Make this runnable with a Heroku button

## Maybe TODO
* frontend interface, Go as a static fileserver, JS tests running
  against stub
* Use 201 created with Location header to force generating internal
  URLs? Return next/prev URLs for pagination in the Location header
  like Greenhouse?
* Generate public URLs for a conversation (maybe have auth package
  support returning a public version of any url?)
* Object creation limits
* Write an example of refactoring list queries to use a read-replica
* Server hosts its own Swagger API docs
* Support rendering conversations as text instead of JSON?
* Generate spans in client or at least take a context for cancellation
* Client with a much more stubbable interface (e.g. Service thing from verificator)
* Client that's more generic so we could use a single client across services

## Maybe include these patterns?
* API clients across multiple languages
* DI?
* Dependency management? It's just being angry about Godep...
