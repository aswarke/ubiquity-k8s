package controller

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/IBM/ubiquity/remote"
	"github.com/IBM/ubiquity/resources"
)

//Controller this is a structure that controls volume management
type Controller struct {
	Client resources.StorageClient
	logger *log.Logger
}

//NewController allows to instantiate a controller
func NewController(logger *log.Logger, storageApiURL string, config resources.UbiquityPluginConfig) (*Controller, error) {

	remoteClient, err := remote.NewRemoteClient(logger, storageApiURL, config)
	if err != nil {
		return nil, err
	}
	return &Controller{logger: logger, Client: remoteClient}, nil
}

//NewControllerWithClient is made for unit testing purposes where we can pass a fake client
func NewControllerWithClient(logger *log.Logger, client resources.StorageClient) *Controller {
	return &Controller{logger: logger, Client: client}
}

//Init method is to initialize the flexvolume, it is a no op right now
func (c *Controller) Init() resources.FlexVolumeResponse {
	c.logger.Println("controller-activate-start")
	defer c.logger.Println("controller-activate-end")

	err := c.Client.Activate()
	if err != nil {
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Plugin init failed %#v ", err),
			Device:  "",
		}

	}

	return resources.FlexVolumeResponse{
		Status:  "Success",
		Message: "Plugin init successfully",
		Device:  "",
	}
}

//Attach method attaches a volume/ fileset to a pod
func (c *Controller) Attach(attachRequest map[string]string) resources.FlexVolumeResponse {
	c.logger.Println("controller-attach-start")
	defer c.logger.Println("controller-attach-end")
	c.logger.Printf("attach-details %#v\n", attachRequest)
	var attachResponse resources.FlexVolumeResponse
	volumeName, exists := attachRequest["volumeName"]
	if !exists {

		attachResponse = resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to attach volume: VolumeName not found : #%v", attachRequest),
			Device:  volumeName,
		}
		c.logger.Printf("Failed-to-attach-volume, VolumeName found %#v ", attachRequest)
		return attachResponse

	}

	c.logger.Printf("Found VolumeName: %s\n", volumeName)
	opts := make(map[string]interface{})

	for key, value := range attachRequest {
		opts[key] = value
	}

	c.logger.Printf("Found opts for attach request: #%v\n", opts)

	_, err := c.Client.GetVolume(volumeName)

	if err != nil {
		if err.Error() == "Volume not found" {
			err = c.Client.CreateVolume(volumeName, opts)
			if err != nil && err.Error() != fmt.Sprintf("Volume `%s` already exists", volumeName) {
				return resources.FlexVolumeResponse{
					Status:  "Failure",
					Message: fmt.Sprintf("Failed to attach volume: %#v", err),
					Device:  volumeName,
				}
			} else if err != nil && err.Error() == fmt.Sprintf("Volume `%s` already exists", volumeName) {
				return resources.FlexVolumeResponse{
					Status:  "Success",
					Message: "Volume already attached",
					Device:  volumeName,
				}
			}
			return resources.FlexVolumeResponse{
				Status:  "Success",
				Message: "Volume attached successfully",
				Device:  volumeName,
			}
		}
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: "Failed checking volume",
			Device:  volumeName}

	}

	return resources.FlexVolumeResponse{
		Status:  "Success",
		Message: "Volume already attached",
		Device:  volumeName,
	}

}

//GetVolumeName method gets a unique volume name cluster wide
func (c *Controller) GetVolumeName(getVolumeNameRequest map[string]string) resources.FlexVolumeResponse {
	c.logger.Println("controller-getvolumename-start")
	defer c.logger.Println("controller-getvolumename-end")
	volumeName, ok := getVolumeNameRequest["volumeName"]
	if !ok {
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: "Failed getting volumeName",
		}
	}
	return resources.FlexVolumeResponse{
		Status:     "Success",
		Message:    "Volume Name retrieved",
		VolumeName: volumeName,
	}
}

//WaitForAttach: Wait for the volume to be attached on the remote node. On success, the path to the device is returned.
func (c *Controller) WaitForAttach(waitForAttachRequest map[string]string) (resources.FlexVolumeResponse, error) {
	c.logger.Println("controller-waitforattach-start")
	defer c.logger.Println("controller-waitforattach-end")
	return resources.FlexVolumeResponse{Attached: true}, nil
}

//IsAttached: Check if the volume is attached on the node.
func (c *Controller) IsAttached(isAttachedRequest map[string]string) (resources.FlexVolumeResponse, error) {
	c.logger.Println("controller-isattached-start")
	defer c.logger.Println("controller-isattached-end")
	return resources.FlexVolumeResponse{Attached: true}, nil
}

//Detach detaches the volume/ fileset from the pod
func (c *Controller) Detach(detachRequest resources.FlexVolumeDetachRequest) resources.FlexVolumeResponse {
	c.logger.Println("controller-detach-start")
	defer c.logger.Println("controller-detach-end")

	c.logger.Printf("detach-details %#v\n", detachRequest)

	// no-op for now, will change with latest flex api update

	return resources.FlexVolumeResponse{
		Status:  "Success",
		Message: "Volume detached successfully",
		Device:  detachRequest.Name,
	}
}

//Mount method allows to mount the volume/fileset to a given location for a pod
func (c *Controller) Mount(mountRequest resources.FlexVolumeMountRequest) resources.FlexVolumeResponse {
	c.logger.Println("controller-mount-start")
	defer c.logger.Println("controller-mount-end")

	mountedPath, err := c.Client.Attach(mountRequest.MountDevice)

	if err != nil {
		c.logger.Printf("Failed to mount volume %#v", err)
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to mount volume %#v", err),
			Device:  "",
		}
	}
	dir := filepath.Dir(mountRequest.MountPath)

	c.logger.Printf("volume/ fileset mounted at %s", mountedPath)

	if _, err = os.Stat(path.Join(mountRequest.MountPath, mountRequest.MountDevice)); err != nil {
		if os.IsNotExist(err) {

			c.logger.Printf("creating volume directory %s", dir)
			err = os.MkdirAll(dir, 0777)
			if err != nil && !os.IsExist(err) {
				return resources.FlexVolumeResponse{
					Status:  "Failure",
					Message: fmt.Sprintf("Failed creating volume directory %#v", err),
					Device:  "",
				}

			}

			symLinkCommand := "/bin/ln"
			args := []string{"-s", mountedPath, mountRequest.MountPath}
			cmd := exec.Command(symLinkCommand, args...)
			_, err = cmd.Output()
			if err != nil {
				c.logger.Printf("Controller: mount failed to symlink %#v", err)
				return resources.FlexVolumeResponse{
					Status:  "Failure",
					Message: fmt.Sprintf("Failed running ln command %#v", err),
					Device:  "",
				}

			}

			return resources.FlexVolumeResponse{
				Status:  "Success",
				Message: fmt.Sprintf("Volume mounted successfully to %s", mountedPath),
				Device:  "",
			}
		}
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed running mount %#v", err),
			Device:  "",
		}
	}

	return resources.FlexVolumeResponse{
		Status:  "Success",
		Message: fmt.Sprintf("Volume mounted successfully to %s", mountedPath),
		Device:  "",
	}

}

//Unmount methods unmounts the volume/ fileset from the pod
func (c *Controller) Unmount(unmountRequest resources.FlexVolumeUnmountRequest) resources.FlexVolumeResponse {
	c.logger.Println("Controller: unmount start")
	defer c.logger.Println("Controller: unmount end")

	volumes, err := c.Client.ListVolumes()
	if err != nil {
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Error finding the volume %#v", err),
			Device:  "",
		}
	}

	volume, err := getVolumeForMountpoint(unmountRequest.MountPath, volumes)
	if err != nil {
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Error finding the volume %#v", err),
			Device:  "",
		}
	}

	err = c.Client.Detach(volume.Name)
	if err != nil && err.Error() != "fileset not linked" {
		return resources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to unmount volume %#v", err),
			Device:  "",
		}
	}

	return resources.FlexVolumeResponse{
		Status:  "Success",
		Message: "Volume unmounted successfully",
		Device:  "",
	}
}

func getVolumeForMountpoint(mountpoint string, volumes []resources.VolumeMetadata) (resources.VolumeMetadata, error) {

	for _, volume := range volumes {
		if volume.Mountpoint == mountpoint {
			return volume, nil
		}
	}
	return resources.VolumeMetadata{}, fmt.Errorf("Volume not found")
}
