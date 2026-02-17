CAMERA_IP ?= 10.0.0.48
CAMERA_PWD ?= 

frontend:
	cd ../frontend && npm install && npm run build

dev: frontend
	goxisbuilder -install -ip $(CAMERA_IP) -sdk "1.15" -ubunutu "22.04" -pwd $(CAMERA_PWD) -ignore ".git build frontend" -tags "dev aarch64" -upx=false -nocopy -watch

build: frontend
	goxisbuilder -install -ip $(CAMERA_IP) -sdk "1.15" -ubunutu "22.04" -pwd $(CAMERA_PWD) -ignore ".git build frontend" -tags "prod aarch64" -upx=false
