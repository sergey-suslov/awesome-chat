# Spec

## Stage 1

Add basic ability to connect to the root room and exchange messages for arbitrary number of users.

### Architecture

UserConnection - module handles input/output of a user. After a connection is created, the user passes a message with a public key, then the user is connected to the main room.
- Subscribes for room's messages
- Subscribes for room's keys/participants updates
- Obtains room's public keys

MessageBroker - module handles communication between users.
- User's incoming messages - stream with encrypted messages (at least once)
- Room's update events (connection/disconnection of a user) - pub/sub channel
- obtain room's pub keys ???

*Varian 1*

User Connection operates as a routing layer with controllers bound to user's input from socket connection. It uses services to operate in pseudo sync/linear manner. User Connection stores the session information.

Services use the information passed from User Connectiona (interface conn {send}, ...) and use MessageBroker service to communicate with other users.

MessageBroker uses NATS to pass messages between the users of the system (request reply technics to operate in pseudo sync manner)

## Stage 2

Add end-to-end encryption to the chat messaging


## Stage 2

Add authorisation and rooms
