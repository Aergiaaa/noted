-- name: GetAllTags :many
select id, name, color 
	from tag 
	where deleted_at is null;

-- name: CreateTag :one
insert into tag (name, color) 
	values ($1, $2) 
	on conflict (name) do update 
		set deleted_at = null, updated_at = now() 
	returning id, name, color;

-- name: DeleteTag :exec
update tag 
	set deleted_at = now() 
	where id = $1;

-- name: RestoreTag :exec
update tag 
	set deleted_at = null 
	where id = $1;

-- name: UpdateTag :one
update tag 
	set name = $1, color = $2, updated_at = now() 
	where id = $3 
	returning id, name, color, updated_at;
