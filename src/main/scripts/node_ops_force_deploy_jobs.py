import command_helper
import context_manager

context_manager.validate_cluster_id()
command_helper.command_remote("sh /opt/agent/bin/force_deploy_jobs.sh")
