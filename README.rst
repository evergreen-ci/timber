======================================
``timber`` -- Cedar Buildlogger Client
======================================

Overview
--------

Timber implements a `grip Sender` backed by Cedar Buildlogger.

See `cedar <https://github.com/evergreen-ci/cedar>`_ and
`grip <https://github.com/mongodb/grip>`_ for more information.


Motivation
----------

Timber enables seamless logging when running tests in
`Evergreen <https://github.com/evergreen-ci/evergreen>`_.


Features
--------

When initializing a Sender with Timber, a gRPC connection to a Cedar backed
application is established. Log lines are buffered and sent over in batches to
Cedar via this gRPC connection. Buffer size, flush intervals, etc. are all
configurable. The Sender is thread-safe.


Code Example
------------

Using the Timber Sender is straightforward, once the logger is setup it can be
passed around anywhere in your code. Log lines are sent using the Send
command: ::

	opts := &timber.LoggerOptions{}
	// populate options struct
	// ...
	l := timber.MakeLogger(ctx, "logging-example", opts)

	l.Send(message.ConvertToComposer(level.Info, "logging is easy!"))
	l.Send(message.ConvertToComposer(level.Debug, "another example"))
        // make sure to close our your logger!
	err := l.Close()


Documentation
-------------

See the `timber godoc <https://godoc.org/github.com/evergreen-ci/timber>`_
for complete documentation.

See the `grip godoc <https://godoc.org/github.com/mongodb/grip/send#Sender>`_
for documentation of the Sender interface.
