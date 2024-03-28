# netboxvmsync


## Project description

Netbox sync helps you keep your Netbox instance up-to-date with changes to your virtualization platform.  It currently has the ability to synchronize Vmware and Proxmox VMs.  It is a single binary golang service that is designed to run nightly.

## Who this project is for

This project is intended for administrators who want to use Netbox as the source of truth, but realize that VMs can and will be added from within the virtualization platform without being entered in Netbox first.


## Project dependencies
Before using netboxvmsync, ensure you have:
* Created a user and API token in Netbox
* Created a user in vmware if using it as the provider
* Created a user and API token in Proxmox if using it


## Instructions for using netboxvmsync
Get started with netboxvmsync by {write the first step a user needs to start using the project. Use a verb to start.}.


### Install netboxvmsync
1. Create the directory to house the application

   ```bash
   sudo mkdir /opt/netboxvmsync
   ```

2. Copy the netboxvmsync binary into the new directory 
 
    ```bash
    sudo cp netboxvmsync /opt/netboxvmsync
    ```

3. Copy the systemd files

   The systemd service and time files can be found in the deploy directory.

   ```bash
   sudo cp deploy/netboxvmsync.* /etc/systemd/system
   sudo systemctl daemon-reload
   sudo systemctl enable netboxvmsync.service
   sudo systemctl enable netboxvmsync.timer
   ```

### Configure netboxvmsync
1. Create the file `/etc/sysconfig/netboxvmsync`
2. Set the following values in the config file:
    - PROVIDER=`{proxmox | vmware}`
    - PROVIDER_URL=
    - PROVIDER_USER=
    - PROVIDER_TOKEN=
    - NETBOX_URL=
    - NETBOX_TOKEN=


### Run netboxvmsync
1. Start the timer

    This will trigger the sync to run at midnight

    ```bash
    sudo systemctl start netboxvmsync.timer
    ```
2. Do a one-time sync run (optional)

    ```bash
    sudo systemctl start netboxvmsync.service
    ```

### Troubleshoot netboxvmsync
1. Logs are written to the system journal
    ```bash
    sudo journalctl -xeu netboxvmsync
    ```
2. These logfiles can also be viewd in /var/log/messages 





