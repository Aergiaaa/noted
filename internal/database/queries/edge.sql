-- name: GetEdges :many
select from_id, from_type, to_id, to_type, link_type from edge;

-- name: GetBacklinks :many
select from_id, from_type, link_type from edge where to_id = $1;

-- name: CreateEdge :exec
insert into edge (from_id, from_type, to_id, to_type, link_type) values ($1, $2, $3, $4, $5);

-- name: DeleteEdge :exec
delete from edge where from_id = $1 and to_id = $2 and link_type = $3;
