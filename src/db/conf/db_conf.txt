# Setting up the database system on your local machine
=======================================================

## 1. Installing PostgreSQL on your local machine
——————————————————————————————————————————————————
http://www.postgres.de/install.html 	(Linux)
http://postgresapp.com/de/		(MacOS)


## 2. Import the SQL-Database
——————————————————————————————
Type in the terminal to:

### 2.1 Create some initial databases and role

$ createuser -P -s -e jannisdikeoulias	(password: analysisbot)
$ createdb -T template0 postgres
$ createdb -T template0 analysisbot

### 2.2 Fill the database (only for the initial import)

$ pg_restore -d analyisbot <Drag the db.dump file here>

### 2.3 Fill the database (database was already imported previously)

$ dropdb analysisbot 
$ pg_restore -C -d postgres <Drag the db.dump file here>


### 2.4 For exporting the database

$ pg_dump -Fc analysisbot > db.dump

===========================================================================EOF