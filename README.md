#GoHTMXChat

This project started [from the gorilla/websocket chat example](https://github.com/gorilla/websocket/tree/main/examples/chat) and expanded from there. The original aim was to convert it into an HTMX-only chat app that would allow anyone to start secure pseudoanonymous group chats.

Features:
- Like any other chat app, you can send and receive messages.
- Shows when a new user joins the chat
- Indicator when someone in the chat is typing
- Shows usernames in chat (shows 'You' for user)
- Users can change usernames - they start off anonymous

Just need to compile or `go run *.go` and it will work on http://127.0.0.1:8080

