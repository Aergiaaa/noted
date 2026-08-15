create table page (
	id uuid primary key default gen_random_uuid(),
	title text not null,
	date date,
	deleted_at timestamptz,
	created_at timestamptz default now(),
	updated_at timestamptz default now()
);
