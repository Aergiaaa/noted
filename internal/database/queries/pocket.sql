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
