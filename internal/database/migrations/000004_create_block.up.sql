create table block (
    id uuid primary key default gen_random_uuid(),
    page_id uuid not null references page(id) on delete cascade,
    parent_block_id uuid references block(id) on delete cascade,
    type text not null,
    content jsonb,
    "order" int not null,
    deleted_at timestamptz,
    created_at timestamptz default now(),
    updated_at timestamptz default now()
);
