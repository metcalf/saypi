# API Documentation

## Common Behaviors

### Authentication

The conversation API requires HTTP Bearer authorization. After creating a user, pass a header of the form `Authorization: Bearer myuserid123`.

### Pagination

Cursor-based, just like the Stripe API. List responses have the
following fields:
* `Type`[string]: Type of object contained in the response.
* `HasMore`[bool]: Whether there are more object available than returned.
* `Data`[array]: Array of objects.

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

*Success Response*: A `mood`

### DELETE /moods/:name

Permanently delete a mood that you created.

*Success Response*: (204 No Content)

### GET /conversations

Returns a list of your conversations.

*Success Response*: A list response of `conversation`s without their `line`s.

### POST /conversations

Creates a new conversation with the specified name for your user account.

*Parameters*
* `heading`[string]: A name for the conversation

*Success Response*: A `conversation`

### GET /conversations/:conversation_id

Retrieves an existing conversation. 

*Success Response*: A `conversation`

### DELETE /conversations/:conversation_id

Deletes the conversation permananently.

### POST /conversations/:conversation_id/lines

Add a new line to the conversation

*Parameters*
* `animal`[string]: Name of the animal to speak.
* `think` [bool]: Whether to show the animal thinking as opposed to speaking.
* `mood`[string]: Customize the tongue and eyes of the animal to its mood.
* `text` [string]: Text for the animal to speak or think.

*Success Response*: A `line`

### GET /conversations/:conversation_id/lines/:line_id

Retrieves a line from the conversation

*Success Response*: A `line`

### DELETE /conversations/:conversation_id/lines/:line_id

Permanently deletes a line from the conversation.

*Success Response*: (204 No Content)

## API objects

### conversation
* `id`[string]
* `heading`[string]
* `lines`[array[line]]

### line

* `id`[string]
* `animal`[string]
* `think` [bool]
* `mood`[string]
* `text`[string]
* `output`[string]: Rendered text of the line.

### mood
* `name`[string]: A unique string name for the mood
* `user_defined`[bool]: Indicates that the mood was created by the user, not built-in.
* `eyes`[string]: A two character string for the animal's eyes.
* `tongue`[string]: A two character string representing the animal's tongue.
