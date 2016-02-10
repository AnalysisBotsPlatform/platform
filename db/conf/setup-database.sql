-- Get shell environment variables.
\set db_user `echo "${DB_USER}"`
\set db_pass `echo "'${DB_PASS}'"`
\set db_name `echo "${DB_NAME}"`

-- Create new user and database.
CREATE ROLE :db_user WITH LOGIN ENCRYPTED PASSWORD :db_pass CREATEDB;
CREATE DATABASE :db_name WITH OWNER :db_user TEMPLATE template0 ENCODING 'UTF8';
GRANT ALL PRIVILEGES ON DATABASE :db_name TO :db_user;

-- Connect to the new database.
\connect :db_name

-- Create all necessary tables.
CREATE TABLE users(
	id SERIAL PRIMARY KEY NOT NULL,
	gh_id integer UNIQUE NOT NULL,
	username varchar(50),
	realname varchar(50),
	email varchar(50),
	token varchar(50) NOT NULL UNIQUE CHECK (token <> ''),
	worker_token varchar(50) NOT NULL UNIQUE CHECK (worker_token <> ''),
	admin boolean
);

CREATE TABLE api_tokens(
	token varchar(50) PRIMARY KEY NOT NULL,
	uid integer NOT NULL,
	name varchar(50) NOT NULL
);

CREATE TABLE api_accesses(
	id SERIAL PRIMARY KEY NOT NULL,
	uid integer NOT NULL,
	time timestamp NOT NULL
);

CREATE TABLE bots(
	id SERIAL PRIMARY KEY NOT NULL,
	name varchar(50) NOT NULL UNIQUE CHECK (name <> ''),
	description text,
	tags varchar(20)[],
	fs_path varchar(100)
);

CREATE TABLE projects(
	id SERIAL PRIMARY KEY NOT NULL,
	gh_id integer UNIQUE NOT NULL,
	name varchar(50) CHECK (name <> ''),
	clone_url varchar(100),
	fs_path varchar(100)
);

CREATE TABLE workers(
	id SERIAL PRIMARY KEY NOT NULL,
	uid integer REFERENCES users(id) NOT NULL,
	token varchar(50) NOT NULL UNIQUE CHECK (token <> ''),
	name varchar(50) NOT NULL,
	last_contact timestamp NOT NULL,
	active boolean NOT NULL,
	shared boolean NOT NULL
);

CREATE TABLE members(
	uid integer REFERENCES users(id) NOT NULL,
	pid integer REFERENCES projects(id) NOT NULL,
	PRIMARY KEY (uid, pid)
);

CREATE TABLE group_tasks(
	id SERIAL PRIMARY KEY NOT NULL,
	uid integer REFERENCES users(id) NOT NULL,
	pid integer REFERENCES projects(id) NOT NULL,
	bid integer REFERENCES bots(id) NOT NULL
);

CREATE TABLE tasks(
	id SERIAL PRIMARY KEY NOT NULL,
	gid integer REFERENCES group_tasks(id) NOT NULL,
	worker_token varchar(50) NOT NULL UNIQUE CHECK (worker_token <> ''),
	start_time timestamp,
	end_time timestamp,
	status integer NOT NULL,
	exit_status integer,
	output text,
	patch varchar(100) NOT NULL
);

CREATE TABLE schedule_tasks(
	id integer UNIQUE REFERENCES group_tasks(id) NOT NULL,
	name varchar(50) NOT NULL CHECK (name <> ''),
	status integer NOT NULL,
	next timestamp,
	cron varchar(100) NOT NULL CHECK (cron <> '')
);

CREATE TABLE onetime_tasks(
	id integer UNIQUE REFERENCES group_tasks(id) NOT NULL,
	name varchar(50) NOT NULL CHECK (name <> ''),
	status integer NOT NULL,
	exec_time timestamp
);

CREATE TABLE instant_tasks(
	id integer UNIQUE REFERENCES group_tasks(id) NOT NULL
);

CREATE TABLE event_tasks(
	id integer UNIQUE REFERENCES group_tasks(id) NOT NULL,
	name varchar(50) NOT NULL CHECK (name <> ''),
	status integer NOT NULL,
	event integer NOT NULL,
	hook_id integer 
);

-- Transfer ownership to the newly created user.
ALTER TABLE users OWNER TO :db_user;
ALTER TABLE api_tokens OWNER TO :db_user;
ALTER TABLE api_accesses OWNER TO :db_user;
ALTER TABLE bots OWNER TO :db_user;
ALTER TABLE projects OWNER TO :db_user;
ALTER TABLE workers OWNER TO :db_user;
ALTER TABLE members OWNER TO :db_user;
ALTER TABLE group_tasks OWNER TO :db_user;
ALTER TABLE tasks OWNER TO :db_user;
ALTER TABLE schedule_tasks OWNER TO :db_user;
ALTER TABLE onetime_tasks OWNER TO :db_user;
ALTER TABLE instant_tasks OWNER TO :db_user;
ALTER TABLE event_tasks OWNER TO :db_user;
