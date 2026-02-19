# Goblocks

## What is it?

**Goblocks is a set of useful Go packages and a simple backend application framework.**

Some packages like `limitmap`, `retry` or `conftool` can be used separately, others have internal dependencies (mostly to the `log` package) and are best used together.

The main thing that joins all these packages together is that they all supposed to work as part of Go backend service providing some API and/or web output. The blend of these packages is the `app` framework package. It allows to create a pretty advanced backend service in a matter of minutes.

Any service built with **Goblocks App** framework contains out of the box:
 - Ready-to-run **HTTP server** with TLS support
 - **Authentication**
 - **Prometheus metrics**
 - **Sentry reporting**
 - **Request rate limiting**
 - **Connection limiting**
 - **CORS support**
 - **Kubernetes health endpoints**
 - Advanced **logging** (based on Zap)
 - **HTTP replying methods**
 - **Cookie-based session support**
 - Simple yet powerful **configuration settings** for all the above

## Getting started

Make yourself familiar with the examples in the `/examples/` dir. Currently there's a [Factorial service](https://github.com/bhmj/goblocks/blob/master/examples/factorial/factorial.go) example that calculates factorials of integers.

All application settings are located in [config.yaml](https://github.com/bhmj/goblocks/blob/master/examples/factorial/config.yaml). The only allowed argument for the binary is a `--config-file=/path/to/config.yaml`.

Look at the [test-factorial.sh](https://github.com/bhmj/goblocks/blob/master/examples/factorial/test-factorial.sh) -- it demostrates the use of the Kubernetes readiness endpoint to verify that the service has started.

Another production-scale example of **Goblocks App** is my [dosasm](https://dosasm.com) [project](https://github.com/bhmj/dosassembly).

## Application config

The configuration for the entire app is located in a single file. The settings are divided into groups: the "app" group and the service groups, where each group name matches the corresponding service name. The service name is specified during service registration.

The "app" config section (the structure is located in [/app/config.go](https://github.com/bhmj/goblocks/blob/master/app/config.go)) covers the most fundamental settings:
   - "http" group defines server params: ports, TLS, auth token, limits and timeouts, metrics;
   - "sentry" group defines Sentry DSN;
   - "logLevel" and "production" define general env settings.

 The service section(s) of the config is totally defined by the user. In the "factorial" example it contains "apiRoot" and "countBits". These are per-service business logic specific parameters.

 If you have multiple services, each service has its own named section in config file.

 The loading of all configuration parameters is completely transparent and automatic, and does not require a single line of code.

 The `yaml` tags in the service configuration may contain **default values** for configuration parameters. This helps drastically reduce the size of the configuration file and minimize noise.

 Configuration values can be automatically taken from environment variables using the `my_key: {{ENV_VARIABLE}}` syntax. This approach combines the best of both worlds: setting parameters via env variables while keeping them organized in a human-readable, structured YAML format.

## Sessions

Starting from v0.5.0 the framework requires the service to have `GetSessionData(SID int) (Data any, error nul)` method. This method is called by the framework middleware for every endpoint having `Options.SIDRequired = true`. The endpoint reads the session data from the context using `httpserver.ContextSessionData` key.

## Considerations and gotchas

- **Important**: The project is still in development mode and not production ready. Breaking changes may occur.
- You don't need to call `r.ParseForm()` in your handlers: the middleware does it. Just use `r.Form["param"]`.

## Breaking changes

 * **v0.5.0**: The default YAML key convention for Config fields is now **camelCase**. Multiple Config YAML keys were modified, config files must be converted.

## Roadmap

 - [x] Basic blocks
 - [x] App framework
 - [x] Config file loading
 - [x] Session tracking
 - [x] DB support with migrations
 - [ ] Templating support
 - [ ] Form support

## Contributing

1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request :)

## Licence

[MIT](http://opensource.org/licenses/MIT)

## Author

Michael Gurov aka BHMJ
