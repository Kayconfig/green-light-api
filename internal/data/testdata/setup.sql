CREATE TABLE IF NOT EXISTS movies (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    year INTEGER NOT NULL,
    runtime integer NOT NULL,
    genres TEXT[] NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP

    CONSTRAINT movies_runtime_check CHECK (runtime >= 0),
    CONSTRAINT movies_year_check CHECK (year BETWEEN 1888 AND date_part('year', now())),
    CONSTRAINT genres_length_check CHECK (array_length(genres, 1) BETWEEN 1 AND 5)

);