create table taggable (
    tag_id uuid not null references tag(id) on delete cascade,
    target_id uuid not null,
    target_type text not null check (target_type in ('page', 'transaction')),
    primary key (tag_id, target_id, target_type)
);
