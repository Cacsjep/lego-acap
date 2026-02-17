frontend:
	cd ../frontend && npm install && npm run build

dev: frontend
	goxisbuilder -install -ip 10.0.0.48 -sdk "1.15" -ubunutu "22.04" -pwd 1qay2wsx -ignore ".git build frontend" -tags "dev aarch64" -upx=false -nocopy -watch

build: frontend
	goxisbuilder -install -ip 10.0.0.48 -sdk "1.15" -ubunutu "22.04"-pwd 1qay2wsx -ignore ".git build frontend" -tags "dev aarch64" -upx=false
