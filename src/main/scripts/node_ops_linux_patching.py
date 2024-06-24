import os
import time

import command_helper
import wait_for_host

command_helper.command_remote("sh /opt/agent/bin/linux_setup.sh")

try:
    r = command_helper.command_remote("""        
        if ! needs-restarting -r >/dev/null; then
            echo "Reboot is needed. Initiating reboot..."
            sudo reboot
        else
            echo "No reboot is needed."
        fi
    """)
    print(r)
except:
    pass

time.sleep(10)

wait_for_host.wait_for_host(target_host=os.getenv("HOST"))