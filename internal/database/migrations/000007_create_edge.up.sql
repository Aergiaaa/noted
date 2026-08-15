create table edge (
    from_id uuid not null,
    from_type text not null check (from_type in ('page', 'transaction')),
    to_id uuid not null,
    to_type text not null check (to_type in ('page', 'transaction')),
    link_type text not null check (link_type in ('wiki-link', 'parent-link', 'finance-link')),
    created_at timestamptz default now(),
    primary key (from_id, to_id, link_type)
);
