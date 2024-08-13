CREATE DATABASE workshop;
\c workshop
CREATE TABLE collections(uid VARCHAR(100) UNIQUE, name VARCHAR(100), success BOOLEAN);
