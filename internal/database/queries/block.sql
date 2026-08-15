-- name: GetBlocksByPage :many
select id, page_id, parent_block_id, type, content, "order" 
	from block 
	where page_id = $1 
		and deleted_at is null 
	order by "order";

-- name: GetRootBlocksByPage :many
SELECT id, page_id, parent_block_id, type, content, "order"
    FROM block
    WHERE page_id = $1
        AND parent_block_id IS NULL
        AND deleted_at IS NULL
    ORDER BY "order";

-- name: GetBlocksByParent :many
SELECT id, page_id, parent_block_id, type, content, "order"
    FROM block
    WHERE parent_block_id = $1
        AND deleted_at IS NULL
    ORDER BY "order";

-- name: CreateBlock :one
insert into block (page_id, parent_block_id, type, content, "order") 
	values ($1, $2, $3, $4, $5) 
	returning id, page_id, parent_block_id, type, content, "order";

-- name: DeleteBlock :exec
update block 
	set deleted_at = now() 
	where id = $1;

-- name: RestoreBlock :exec
update block 
	set deleted_at = null 
	where id = $1;

-- name: UpdateBlock :one
update block 
	set type = $1, content = $2, updated_at = now() 
	where id = $3 
	returning id, type, content, updated_at;

-- name: ReorderBlocks :exec
update block
	set "order" = @new_order, updated_at = now()
	where id = @id;
