package storage

const createTablesSQL = `
CREATE TABLE IF NOT EXISTS Users
(
	id      SERIAL PRIMARY KEY,
	login 	VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS Orders
(
    id SERIAL PRIMARY KEY,
    order_id varchar(50) UNIQUE,
    status varchar(15),
	amount REAL CHECK(amount >= 0),
	uploaded_at timestamp with time zone,
	processed_at timestamp with time zone,
	user_id INTEGER REFERENCES Users(id)
);

CREATE TABLE IF NOT EXISTS Withdrawn
(
    id SERIAL PRIMARY KEY,
    order_id varchar(50) UNIQUE,
    amount REAL CHECK(amount >= 0),
	uploaded_at timestamp with time zone,
	user_id INTEGER REFERENCES Users(id)
);

CREATE TABLE IF NOT EXISTS Balance
(
	id      SERIAL PRIMARY KEY,
	balance REAL CHECK(balance >= 0),
	withdrawn REAL CHECK(withdrawn >= 0),
	user_id INTEGER UNIQUE REFERENCES Users(id)
);
`
