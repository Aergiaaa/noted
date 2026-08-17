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
			from_pocket_id = coalesce($5, from_pocket_id), 
			to_pocket_id = coalesce($6, to_pocket_id), 
			updated_at = now() 
	where id = $7 
	returning id, title, type, amount, date, from_pocket_id, to_pocket_id, updated_at;

-- name: GetDeletedTransactions :many
select id, title, type, amount, date, deleted_at
	from transaction
	where deleted_at is not null
	order by deleted_at desc
	limit 50;

-- name: GetTransactionsByFilter :many
select
	t.id,
	t.title,
	t.type,
	t.amount,
	t.date,
	t.from_pocket_id,
	t.to_pocket_id,
	fp.name as from_pocket_name,
	tp.name as to_pocket_name
from transaction t
left join pocket fp on fp.id = t.from_pocket_id
left join pocket tp on tp.id = t.to_pocket_id
where t.deleted_at is null
	and (sqlc.arg('pocket_id')::uuid is null or t.from_pocket_id = sqlc.arg('pocket_id') or t.to_pocket_id = sqlc.arg('pocket_id'))
	and (sqlc.arg('from_date')::date is null or t.date >= sqlc.arg('from_date'))
	and (sqlc.arg('to_date')::date is null or t.date <= sqlc.arg('to_date'))
order by t.date desc, t.created_at desc
limit 50;

-- name: GetTransactionsWithPocketNames :many
select
	t.id,
	t.title,
	t.type,
	t.amount,
	t.date,
	t.from_pocket_id,
	t.to_pocket_id,
	fp.name as from_pocket_name,
	tp.name as to_pocket_name
from transaction t
left join pocket fp on fp.id = t.from_pocket_id
left join pocket tp on tp.id = t.to_pocket_id
where t.deleted_at is null
order by t.date desc, t.created_at desc
limit 50;

-- name: GetMonthlySummary :one
select
	coalesce(sum(case when type = 'income' then amount end), 0)::numeric as income,
	coalesce(sum(case when type = 'expense' then amount end), 0)::numeric as expense,
	coalesce(sum(case when type = 'transfer' then amount end), 0)::numeric as transfer
	from transaction
	where deleted_at is null
		and date >= date_trunc('month', now())
		and date < date_trunc('month', now()) + interval '1 month';
