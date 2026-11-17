-- +goose Up
-- Keep exact old keys for rollback. IDs, owners, chat, sessions and staff access stay intact.
CREATE TABLE mvp2_room_locations (
    room_id uuid PRIMARY KEY REFERENCES rooms(id) ON DELETE CASCADE,
    building_key text,
    map_key text NOT NULL
);
INSERT INTO mvp2_room_locations (room_id, building_key, map_key)
SELECT id, building_key, map_key FROM rooms WHERE map_key <> 'campus';

UPDATE rooms SET building_key = CASE building_key
    WHEN 'kelas-a' THEN 'angka'
    WHEN 'kelas-b' THEN 'kelas'
    WHEN 'perpustakaan' THEN 'perpus'
    WHEN 'aula' THEN 'galeri'
    WHEN 'kantin' THEN 'studio'
    WHEN 'lapangan' THEN 'arena'
    ELSE building_key END
WHERE map_key <> 'campus';
UPDATE rooms SET map_key = 'baca' WHERE building_key = 'perpus';
-- ruang-guru retains its restricted key but uses the new classroom/discussion visuals.

-- +goose Down
UPDATE rooms r SET building_key = old.building_key, map_key = old.map_key
FROM mvp2_room_locations old WHERE old.room_id = r.id;
-- Translate rooms created after Up back to supported MVP 1 locations/layouts.
UPDATE rooms SET building_key = CASE building_key
    WHEN 'angka' THEN 'kelas-a' WHEN 'kelas' THEN 'kelas-b'
    WHEN 'perpus' THEN 'perpustakaan' WHEN 'galeri' THEN 'aula'
    WHEN 'studio' THEN 'kantin' WHEN 'arena' THEN 'lapangan'
    WHEN 'sains' THEN 'kelas-a' WHEN 'alun-alun' THEN 'taman'
    ELSE building_key END,
    map_key = CASE WHEN map_key IN ('lab', 'baca') THEN 'classroom' ELSE map_key END
WHERE id NOT IN (SELECT room_id FROM mvp2_room_locations) AND map_key <> 'campus';
DROP TABLE mvp2_room_locations;
