## Meta Logger

This implements [zap](https://pkg.go.dev/go.uber.org/zap)-based meta logger with buffering capability, allowing to merge multiple log records into one log line.

### Usage

```Go
import "github.com/workato/engine-packages/log"

logLevel := "info"
oneLiner := false
logger, err := log.New(logLevel, oneLiner)
logger.Info("my message", log.String("dummy", "value"), log.Bool("noted", true))
```

### One line logging

This mode is intended to decrease the load on log processing subsystems and help to reduce disk space requirements in case of intensive logging when you cannot just enable sampling, for system observability reasons.

With `oneLiner == false` the logger works just like a normal zap logger.

With `oneLiner == true` it aggregates log data in memory until `logger.Flush()` is called, then it outputs a single log record. Suppose we have the following calls:

```Go
logger.Info("loading", log.String("dummy", "foo"))
// ...
logger.Info("operation", log.String("status", "good"))
// ...
logger.Warn("system", log.Int("memory low", 33))
// ...
logger.Flush() // MUST EXPLICITLY FLUSH
```

If we set our logger to normal mode, like this:
```Go
logger, err := log.New("info", false) // normal mode
```

we will get 3 separate log records:

```
{ "level":"info", "time":"...", "msg":"loading", "dummy":"foo" }
{ "level":"info", "time":"...", "msg":"operation", "status":"good" }
{ "level":"warn", "time":"...", "msg":"system", "memory low":33 }
```

However, if we set our logger to "oneliner" mode, like this:
```Go
logger, err := log.New("info", true) // oneliner mode
```

we will get a single log record:

```
{ "level":"warn", "time":"...", "msg":"system", "dummy":"foo", "status":"good", "memory low":33 }
```

Note that the whole log record has "warn" type as it is the highest level of all.

### Main message in oneliner mode

When merging multiple log records into a single one we need to set a message (`msg` field) for the whole record. Naturally you want it to be the most informative among all records, for example it may be the type of the endpoint currently working or something similar. If nothing is specified explicitly the **first message** of the highest level is used.

However, since we can switch oneliner mode on and off, we may want to explicitly mark the main message which should be selected amonth other messages of the same level. You may do it using `MainMessage()` method:

```Go
logger.Info("start request", log.String("connections", conns))
// ...
logger.Info("my operation", log.String("status", status), log.MainMessage()) // <--
// ...
logger.Info("stop request", log.Float("duration", dur), log.Int("items processed", nItems))
// ...
logger.Flush()
```

the result in oneliner mode will be:

```
{ "level":"info", "time":"...", "msg":"my operation", "connections": 20, "status":"good", "duration":"0.004", "items processed":6 }
```

In the normal mode we will just get three separate log lines without any extra fields.

### Verbose (normal multi-line) mode

Sometimes the "oneliner" mode is not suitable for certain uses, like, for example, logging in multiple goroutines running in parallel. In this case there's a possibility of merging log records from different goroutines and the result log line can be messy. For such cases the `Verbose()` method is used which returns a normal multi-line logger, even if the logging has been initially switched to a one-line mode. So, you can have a "oneliner" logger for your request handler and "verbose" logger for background processes.
