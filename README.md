# Analysis Bots Platform for GitHub

[![GoDoc](https://godoc.org/github.com/AnalysisBotsPlatform/platform?status.svg)](https://godoc.org/github.com/AnalysisBotsPlatform/platform)

Did you ever want to try a cool new project on one of your GitHub projects but
did not want to go through the hassle of installing it? Are you bored of doing
repetitive tasks over and over again while working on a project? Do you have any
other problem that you want to have solved in an automated fashion? Then you
should definitely check out this project.

The Analysis Bots Platform for GitHub enables its users to solve all of the
problems above and a lot more. It provides a clean looking and easy to use user
interface. You do not need to sign up for a new service as you can just use your
already existing GitHub account.

You want to try it out? It is as easy as signing in with your GitHub account.
All your projects are imported and synchronized automatically. The Analysis Bots
Platform provides the rest.

Is there something you want to have but is not available yet? Do not hesitate,
implement a Bot that does what you want and share it with everyone else. It is
as easy as setting up a Docker image.

But wait a minute. You might ask yourself now: What exactly am I able to do with
this platform? Great question! For the simplest scenario you choose one of your
projects and select a Bot provided by the platform. This might be a Bot that
checks for typos and spelling errors in your comments. Then the platform pulls
your project and the Bot with all its dependencies and executes it on your
project. Finally, you are presented the results of the execution. There are more
sophisticated mechanisms in place like Bot execution automation and creation of
pull request for encountered problems.

What is left to say? Go on and give it a try!


# Installation

The following instructions are meant for manual installation. It is recommended
to use the installation process using Docker. You can check it out
[here](https://github.com/AnalysisBotsPlatform/easy-install/).

## Prerequisites

These instructions assume that you have access to a working PostgreSQL server.
For further information visit http://postgresql.org.

Before you start, make sure you have configured Go correctly. Especially verify
that the `GOPATH` environment variable is set properly.

## Setup environment

To start the installation you need to clone this repository. So run
```shell
git clone https://github.com/AnalysisBotsPlatform/platform <destination>
```
where you replace `<destination>` by the directory in which the repository
should be cloned into. Now use `cd` to enter this directory, i.e. run
```shell
cd <destination>
```

Next the `setup_env.sh` file must be edited. You can do so by using your
favorite text editor. Any one will do. The file should contain sufficient
explanation but here is a more thorough description:

| Variable        | Content                                                       |
| --------------- | ------------------------------------------------------------- |
| `CLIENT_ID`     | GitHub Client ID                                              |
| `CLIENT_SECRET` | GitHub Client Secret                                          |
| `SESSION_AUTH`  | Random string used to identify the session                    |
| `SESSION_ENC`   | Random string used to encrypt the session                     |
| `CACHE_PATH`    | File system path where the platform may store files           |
| `ADMIN_USER`    | GitHub user name of the person who administrates the platform |
| `APP_PORT`      | Port where the application is reachable                       |
| `APP_SUBDIR`    | URL path where the applications is reachable                  |
| `WORKER_PORT`   | Port where the communication interface for workers is exposed |
| `DB_HOST`       | Host name where the PostgreSQL database is located            |
| `DB_USER`       | User that is used to access the PostgreSQL database           |
| `DB_PASS`       | Password that is used to access the PostgreSQL database       |
| `DB_NAME`       | Name of the database that is used to store the platforms data |

The values for the `CLIENT_*` variables can be found under the Applications
Settings page on http://github.com. In case you have not already created an
application for the Analysis Bot Platform you can just go on and create a new
one. Type in any name you like, for example "Analysis Bots Platform for GitHub"
and enter `http://localhost:$APP_PORT` for both the Homepage URL and the
Authorization callback URL if you want to run the platform locally or an
existing URL under which the application is accessible. If you run it on a
web server you can make it accessible using a `.htaccess` file for example. Just
rewrite accesses to the platform so that they use port `APP_PORT`. API accesses
(i.e.  accesses to `<URL you chose>/api/*`) should by rewritten to use the port
you specified in `WORKER_PORT`.

You can use
```shell
date | md5sum
```
to generate random strings that can be used for the `SESSION_*` variables.
Anyhow, make sure that both are exactly 32 characters long. ATTENTION: the
strings you enter for these variables are used to encrypt the user cookies.
Changing these will invalidate all user cookies.

As you are going to create a new database and a user for it, make sure to set
`DB_NAME` and `DB_USER` to values that are not already taken.

To finish the environment setup simply run
```shell
source setup_env.sh
```

## Webapp installation

To install the platform run the following commands:
```shell
go get -d -v
```
This will download this project and all its dependencies so Go is able to work
with it.

## Setup Database

Assuming that your database instance runs locally (in the sense that it is the
machine you are working on for the steps before), setting up the database is
straight forward:
```shell
psql -U <username> -f db/conf/setup-database.sql
```
It is unlikely that you are working as the user that has direct access to the
database. Thus specify a `<username>` that has.


# Usage

In order to run the platform you need to navigate to the directory where you
cloned the project to. The following command should do the trick:
```shell
cd <destination>
```
where you replace `<destination>` by the directory from the installation steps.

Now you can run
```shell
go run main.go
```
and an instance of the platform should be reachable on port `APP_PORT`.

Try it out by visiting `http://localhost:$APP_PORT` with your web browser if you
run the platform on your local machine or by visiting the URL you used during
the installation. Congratulations, you successfully installed and started your
own instance of the Analysis Bots Platform for GitHub!


# FAQ

## Is there a sample `.htaccess` file that I can use?

Yes, there is. You can find it [here](http://example.com)


# License

TODO add license information
