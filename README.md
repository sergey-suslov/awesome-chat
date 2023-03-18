# Chat 💬

End-to-end encrypted chat. No DB. Nats as message broker.
Encryption is done using RSA public-key cryptosystem.

# TODO

- Use AES for messages encryption and RSA for initial secret exchange.
- Add a nice interface for client app (Bubble Tea TUI)
- Add mechanism for user key invalidation (for NATS users store) for ungracefully failed services.
