Flannel upgrade/downgrade procedure
 
There are two ways of changing flannel version in the running cluster:
 
*1. Remove old resources definitions and install a new one.*
* Pros: Cleanest way of managing resources of the flannel deployment and no manual validation required as long as no additional resources was created by administrators/operators
* Cons: Massive networking outage within a cluster during the version change
 
To follow that approach one just needs to have a definition of the current version of flannel and the new one. kubectl delete -f <old>.yaml and kubectl create -f <new>.yaml will do the thing
 
*2. On the fly version*
* Pros: Less disruptive way of changing flannel version, easier to do
* Cons: Some version may have changes which can't be just replaced and may need resources cleanup and/or rename, manual resources comparison required
 
To do on the fly update both old and new yaml should be compared. Resources in the new yaml with the same name or new ones can be left as is. Resources which exist in old yaml but are missing in the new yaml need to be removed right after the version change. After all of the changes are validated one can just use kubectl create -f <new>.yaml and check that version changed. After that resources which are missing a new applied version can be safely deleted.

