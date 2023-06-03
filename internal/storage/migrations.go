package storage

const createTablesSQL = `
CREATE TABLE IF NOT EXISTS Users
(
	id      SERIAL PRIMARY KEY,
	login 	VARCHAR(50) UNIQUE,
    password VARCHAR(50)
);

CREATE TABLE IF NOT EXISTS Orders
(
    id SERIAL PRIMARY KEY,
    order_id varchar(50),
    status varchar(15),
	amount INTEGER,
	uploaded_at timestamp with time zone,
	processed_at timestamp with time zone,
	user_id INTEGER REFERENCES Users(id)
);

CREATE TABLE IF NOT EXISTS Withdrawn
(
    id SERIAL PRIMARY KEY,
    order_id varchar(50),
    amount INTEGER,
	uploaded_at timestamp with time zone,
	user_id INTEGER REFERENCES Users(id)
);

CREATE TABLE IF NOT EXISTS Balance
(
	id      SERIAL PRIMARY KEY,
	balance INTEGER,
	withdrawn INTEGER,
	user_id INTEGER REFERENCES Users(id)
);
`
