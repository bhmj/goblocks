# Goblocks

## What is it?

**It's a set of useful Go packages and a simple backend application framework.**

Some packages like `limitmap`, `retry` or `conftool` can be used separately, others have internal dependencies (mostly to the `log` package) and are best used together.

The main thing that joins all these packages together is that they all supposed to work as part of Go backend service providing some API and/or web output. The blend of these packages is the `app` framework package. IT allows to create a pretty advanced backend service in a matter of minutes.

Any service built with Goblocks app framework contains out of the box:
 - Ready-to-run **HTTP server** with TLS support
 - **Authentication**
 - **Prometheus metrics**
 - **Sentry reporting**
 - **Request rate limiting**
 - **Connection limiting**
 - **Kubernetes health endpoints**
 - Advanced **logging** (based on Zap)
 - **HTTP replying methods**
 - Simple yet powerful **configuration settings** for all the above

## Getting started

Make yourself familiar with the examples in /examples/ dir. Currently there's a **"Factorial"** service example which calculates factorials of integers.

The settings for the whole app are located in config.yaml. The only allowed argument for the binary is a `--config-file=/path/to/config.yaml`.

Look at the `test-factorial.sh`. It demostrates the use of k8s readiness endpoint to check for the service readiness.

Another production-scale example of Goblocks App is my [dosasm](https://dosasm.com) [project](https://github.com/bhmj/dosassembly).

## Application config

The config for the whole app is located in a single file. The settings are divided in "groups": the "app" group and the services groups. The group name is the service same. The service name is specified during service registration.

The "app" config section (the structure is located in `/app/config.go`) covers the most fundamental settings:
   - "http" group defines server params: ports, TLS, auth token, limits and timeouts, metrics;
   - "sentry" group defines Sentry DSN;
   - "log_level" and "production" define general env settings.

 The service section(s) of the config is totally defined by the user. In the "factorial" example it contains "api_base" and "count_bits". These are per-service business logic specific parameters.

 If you have multiple services, each service has its own named section in config file.

 The loading of all the configuration parameters is completely transparent and automatic and does not requre a single line of code. 

 The `yaml` tags in service config may contain **default values** for configuration parameters. This helps to drastically decrease the size of the config file and reduce the noise.

 The config values can be automatically taken from environment variables: `my_key: {ENV_VARIABLE}`. This takes the best from both worlds: setting parameters via env variables and organizing them in a human-readable structured format (yaml).

## Roadmap

 - [x] Basic blocks
 - [x] App framework
 - [x] Config file loading
 - [ ] Session tracking
 - [ ] DB support with migrations
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
