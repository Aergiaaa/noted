create table transaction (
    id uuid primary key default gen_random_uuid(),
    title text not null,
    type text not null check (type in ('income', 'expense', 'transfer')),
    amount numeric(12,2) not null,
    date date not null,
    from_pocket_id uuid references pocket(id) on delete set null,
    to_pocket_id uuid references pocket(id) on delete set null,
    deleted_at timestamptz,
    created_at timestamptz default now(),
    updated_at timestamptz default now()
);
