create table pocket (
	id uuid primary key default gen_random_uuid(),
	name text not null,
	type text not null,
	deleted_at timestamptz,
	created_at timestamptz default now(),
	updated_at timestamptz default now()
);
