IMAGE="maand"

build:
	docker build -t $(IMAGE) ./src/main

initialize:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) initialize

run_command_no_check:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) run_command_no_check $(ROLE)

run_command:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) run_command $(ROLE)

run_command_local:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) run_command_local $(ROLE)

update:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) update

deploy-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) deploy_jobs

force-deploy-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) force_deploy_jobs

stop-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) stop_jobs

restart-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) restart_jobs

rolling-upgrade:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) rolling_upgrade

health-check:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) health_check
