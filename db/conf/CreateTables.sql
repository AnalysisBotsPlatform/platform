CREATE TABLE users(
	id SERIAL PRIMARY KEY NOT NULL,
	gh_id integer UNIQUE NOT NULL,
	username varchar(50),
	realname varchar(50),
	email varchar(50),
	token varchar(50) NOT NULL UNIQUE, CHECK (token <> ''),
	worker_token varchar(50) NOT NULL UNIQUE, CHECK (worker_token <> ''),
	admin boolean
);

CREATE TABLE bots(
	id SERIAL PRIMARY KEY NOT NULL,
	name varchar(50) NOT NULL UNIQUE, CHECK (name <> ''),
	description text,
	tags varchar(20)[],
	fs_path varchar(100)
);

CREATE TABLE projects(
	id SERIAL PRIMARY KEY NOT NULL,
	gh_id integer UNIQUE NOT NULL,
	name varchar(50), CHECK (name <> ''),
	clone_url varchar(100),
	fs_path varchar(100)
);

CREATE TABLE workers(
	id SERIAL PRIMARY KEY NOT NULL,
	uid integer REFERENCES users(id) NOT NULL,
	token varchar(50) NOT NULL UNIQUE, CHECK (token <> ''),
	name varchar(50) NOT NULL,
	last_contact timestamp NOT NULL,
	active boolean NOT NULL,
	shared boolean NOT NULL
);

CREATE TABLE tasks(
	id SERIAL PRIMARY KEY NOT NULL,
	uid integer REFERENCES users(id) NOT NULL,
	pid integer REFERENCES projects(id) NOT NULL,
	bid integer REFERENCES bots(id) NOT NULL,
	start_time timestamp,
	end_time timestamp,
	status integer NOT NULL,
	exit_status integer,
	output text
);

CREATE TABLE members(
	uid integer REFERENCES users(id) NOT NULL,
	pid integer REFERENCES projects(id) NOT NULL,
	PRIMARY KEY (uid, pid)
);
