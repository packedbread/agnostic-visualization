### global code todo and fixme

- read up on `context` package, maybe reuse context in several requests and/or use incoming request context


### general ideas

- separate authenticator into read and write authenticators, read for frontend (poll requests) and write for scene modification requests
- look into using rpc code in returned errors
- tls, musthave
- re-poll for new drawings immediately after getting non-empty response to get the whole scene faster