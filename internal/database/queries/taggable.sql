-- name: AttachTag :exec
insert into taggable (tag_id, target_id, target_type) 
	values ($1, $2, $3);

-- name: DetachTag :exec
delete from taggable 
	where tag_id = $1 
		and target_id = $2 
		and target_type = $3;

-- name: GetTagsByTarget :many
select t.id, t.name, t.color 
	from tag t
	inner join taggable tg 
		on t.id = tg.tag_id
	where tg.target_id = $1 
		and tg.target_type = $2;

-- name: GetPagesByTag :many
select p.id, p.title
	from page p
	inner join taggable tg on tg.target_id = p.id
	where tg.tag_id = $1
		and tg.target_type = 'page'
		and p.deleted_at is null
	order by p.updated_at desc
	limit 50;
