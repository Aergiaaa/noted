-- name: GetAllPages :many
select id, title, date 
	from page 
	where deleted_at is null;

-- name: GetPageByID :one
select id, title, date 
	from page 
	where id = $1 
		and deleted_at is null;

-- name: CreatePage :one
insert into page (title, date) 
	values ($1, $2) 
	returning id, title, date;

-- name: DeletePage :exec
update page 
	set deleted_at = now() 
	where id = $1;

-- name: RestorePage :exec
update page 
	set deleted_at = null 
	where id = $1;

-- name: UpdatePage :one
update page 
	set title = $1, date = $2, updated_at = now() 
	where id = $3 
	returning id, title, date, updated_at;

-- name: GetPagePaginated :many
select id,title
	from page
	where deleted_at is null
	order by updated_at desc
	limit $1 offset $2;

-- name: CountPages :one
select count(*)::int
	from page
	where deleted_at is null;
