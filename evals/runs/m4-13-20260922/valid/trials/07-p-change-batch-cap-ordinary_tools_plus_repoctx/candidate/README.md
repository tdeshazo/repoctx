# Relay Board

Relay Board prepares outgoing status reports for dispatch.

## Operations

The dispatch service waits two minutes before retrying a queued report. Queue
selection and recipient-batch limits are implemented in the `relay` package.
