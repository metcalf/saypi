# saypi
Cowsay API to demonstrate several golang design patterns

# API

## Common Behaviors

### Authentication

The conversation API requires HTTP Bearer authorization. After creating a user, pass a header of the form `Authorization: Bearer myuserid123`.

### Errors

TODO: Error codes
TODO: Rate limits, object creation limits.


### Pagination

TODO

## Endpoints

### POST /users

Create a new user for authenticating with the conversation API.

*Parameters* (none)

*Success Response*
* `id`[string]: A user ID that serves as an authentication token for the API

### GET /users/:id

Returns a response indicating if the provided user ID exists (204) or not (404).

### GET /animals

Return a list of available animals for conversations.

*Success Response*
* `animals`[array]: Array of animal name strings.

### GET /moods

Return a list of available moods with which to customize the eyes and
tongue of your animals.

*Success Response*: A list response of `mood`s

### PUT /moods/:name

Create or update a mood. You can only update moods you created.

*Parameters*
* `eyes`[string]: A two character string for the animal's eyes.
* `tongue`[string]: A two character string representing the animal's tongue.

### GET /moods/:name

Retrieve an existing mood.

*Success Response*: A mood

### DELETE /moods/:name

Permanently delete a mood that you created.

### GET /conversations

Returns a list of your conversations.

*Success Response*: A list response of `conversation`s without their `line`s.

### PUT /conversations/:name

Creates a new conversation with the specified name for your user account.

*Parameters* (none)

*Success Response*: A `conversation`

### GET /conversations/:name

Retrieves an existing conversation. 

*Success Response*: A `conversation`

### DELETE /conversations/:name

Deletes the conversation permananently.

### POST /conversations/:name/lines

Add a new line to the conversation

*Parameters*
* `animal`[string]: Name of the animal to speak.
* `mood`[string]: Customize the tongue and eyes of the animal to its mood. 

### GET /conversations/:name/lines/:id

Retrieves a line from the conversation

*Success Response*: A line

### DELETE /conversations/:name/lines/:id

Permanently deletes a line from the conversation.

## API objects

### conversation
* `name`[string]
* `public_url`[string]: URL where the conversation may be accessed in text form without authentication.
* `animals`[array[string]]
* `lines`[array[line]]

### line

* `id`[string]
* `animal`[string]
* `mood`[string]
* `text`[string]: Rendered text of the line.

### mood
* `name`[string]: A unique string name for the mood
* `user_created`[bool]: Indicates that the mood was created by the user, not built-in.
* `eyes`[string]: A two character string for the animal's eyes.
* `tongue`[string]: A two character string representing the animal's tongue.

# Notes
* Use 201 created to force generating internal URLs?
* Stubbing?
* Package descriptions
* How would a metrics package get the route pattern?
