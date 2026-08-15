-- name: GetAllPockets :many
select id, name, type, deleted_at, updated_at
	from pocket 
	where deleted_at is null;

-- name: CreatePocket :one
insert into pocket (name, type) 
	values ($1, $2) 
	returning id, name, type, created_at, updated_at;

-- name: DeletePocket :exec
update pocket 
	set deleted_at = now() 
	where id = $1;

-- name: RestorePocket :exec
update pocket 
	set deleted_at = null 
	where id = $1;

-- name: UpdatePocket :one
update pocket 
	set name = $1, type = $2, updated_at = now() 
	where id = $3 
	returning id, name, type, updated_at;

-- name: GetPocketBalances :many
select
	p.id,
	p.name,
	p.type,
	coalesce(sum(
		case
			when t.type = 'expense' and t.from_pocket_id = p.id then -t.amount
			when t.type = 'income' and t.to_pocket_id = p.id then t.amount
			when t.type = 'transfer' and t.to_pocket_id = p.id then t.amount
			when t.type = 'transfer' and t.from_pocket_id = p.id then -t.amount
			else 0
		end
	), 0)::numeric as balance
from pocket p
left join transaction t
	on (t.from_pocket_id = p.id or t.to_pocket_id = p.id)
	and t.deleted_at is null
where p.deleted_at is null
group by p.id
order by p.updated_at desc;
