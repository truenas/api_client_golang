./truenas_go -url ws://192.168.1.149/api/current -api-key=${TRUENAS_API_KEY} --timeout 20 --method core.ping_remote --params '[{"hostname": "192.168.1.70"}]'
./truenas_go -url ws://192.168.1.149/api/current -api-key=${TRUENAS_API_KEY} --timeout 200 --job --method app.upgrade --params '["dockge"]'
./truenas_go -url ws://192.168.1.149/api/current --api-key=${TRUENAS_API_KEY} --timeout 200 --method zfs.snapshot.query --params '[ [["dataset", "=", "space/dt"]] ]'
./truenas_go -url wss://192.168.1.149/api/current --verifyssl=false -api-key=${TRUENAS_API_KEY} --timeout 200 --job --method app.upgrade --params '["dockge"]'


