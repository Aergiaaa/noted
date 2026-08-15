-- name: GetAllTransactions :many
select id, title, type, amount, date, from_pocket_id, to_pocket_id 
	from transaction 
	where deleted_at is null;

-- name: GetTransactionByID :one
select id, title, type, amount, date, from_pocket_id, to_pocket_id 
	from transaction 
	where id = $1 
		and deleted_at is null;

-- name: CreateTransaction :one
insert into transaction (title, type, amount, date, from_pocket_id, to_pocket_id) 
	values ($1, $2, $3, $4, $5, $6) 
	returning id, title, type, amount, date, from_pocket_id, to_pocket_id;

-- name: DeleteTransaction :exec
update transaction 
	set deleted_at = now() 
	where id = $1;

-- name: RestoreTransaction :exec
update transaction 
	set deleted_at = null 
	where id = $1;

-- name: UpdateTransaction :one
update transaction 
	set title = $1, 
			type = $2, 
			amount = $3, 
			date = $4, 
			from_pocket_id = $5, 
			to_pocket_id = $6, 
			updated_at = now() 
	where id = $7 
	returning id, title, type, amount, date, from_pocket_id, to_pocket_id, updated_at;

-- name: GetTransactionsWithPocketNames :many
select
	t.id,
	t.title,
	t.type,
	t.amount,
	t.date,
	fp.name as from_pocket_name,
	tp.name as to_pocket_name
from transaction t
left join pocket fp on fp.id = t.from_pocket_id
left join pocket tp on tp.id = t.to_pocket_id
where t.deleted_at is null
order by t.date desc, t.created_at desc
limit 50;
