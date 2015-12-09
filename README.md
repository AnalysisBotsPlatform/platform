Analysis Bots Platform for GitHub
=================================

[![GoDoc](https://godoc.org/github.com/AnalysisBotsPlatform/platform?status.svg)](https://godoc.org/github.com/AnalysisBotsPlatform/platform)

TODO fill in some description


Installation
============

Before you start make sure you have configured Go correctly. Especially verify
that the `GOPATH` variable is set properly.

Than you can simply run
```shell
go get "github.com/AnalysisBotsPlatform/platform"
```
and all the code is fetched from GitHub. This includes all of the projects
dependencies.

Finally, you need a working PostgreSQL server running on your local machine. For
installation instructions visit http://postgresql.org. After successfully
installing PostgreSQL create a database called `analysisbots` by running the
command as the database user (most likely `postgres`):
```shell
createdb analysisbots
```


Usage
=====

In order to run the platform locally you need to navigate to the directory where
Go fetched the source files to. The following command should do the trick:
```shell
cd "${GOPATH}/src/github.com/AnalysisBotsPlatform/platform"
```

Before you can start your local instance you need to set some environment
variables using `export VAR_NAME=VALUE`. These are

| Variable        | Content                                             |
| --------------- | --------------------------------------------------- |
| `CLIENT_ID`     | GitHub Client ID                                    |
| `CLIENT_SECRET` | GitHub Client Secret                                |
| `SESSION_AUTH`  | Random string used to identify the session          |
| `SESSION_ENC`   | Random string used to encrypt the session           |
| `DB_USER`       | User that is used to access the PostgreSQL database |
| `DB_PASS`       | Password that is used to access the database        |

The values for the `CLIENT_*` variables can be found under the Applications
Settings page on http://github.com. In case you have not already created an
application for the Analysis Bot Platform you can just go on and create a new
one. Type in any name you like, for example "Analysis Bots Platform for GitHub"
and enter `http://localhost:8080` for both the Homepage URL and the
Authorization callback URL.

You can use
```shell
date | md5sum
```
to generate random strings that can be used for the `SESSION_*` variables. Make
sure to save these strings for later use since closing the terminal deletes all
environment variables and the old cookies storing the session information are
invalidated.

Now you can run
```shell
go run main.go
```
and a local instance of the platform should be reachable on port 8080.

Try it out by visiting [http://localhost:8080](http://localhost:8080) with your
web browser. Congratulations, you successfully installed and started you local
instance of the Analysis Bots Platform for GitHub!


License
=======

TODO add license information
