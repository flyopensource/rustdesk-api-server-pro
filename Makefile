main_output=build
frontend=soybean-admin
frontend_dist=${frontend}/dist

build: clean
	mkdir -p ${main_output}
	CGO_ENABLED=0 go build -C backend -trimpath -ldflags="-s -w" -o ../${main_output}/
	cd ${frontend} && pnpm build
	cp -R ${frontend_dist} ${main_output}/dist
	cp backend/server.yaml ${main_output}/server.yaml

clean:
	rm -rf ${main_output}
	rm -rf ${frontend_dist}
	go clean
