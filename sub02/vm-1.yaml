apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: argo-test
      fieldsType: FieldsV1
      fieldsV1:
        'f:status':
          'f:preferenceRef':
            .: {}
            'f:controllerRevisionRef':
              .: {}
              'f:name': {}
            'f:kind': {}
            'f:name': {}
          'f:printableStatus': {}
          'f:runStrategy': {}
          'f:conditions': {}
          .: {}
          'f:instancetypeRef':
            .: {}
            'f:controllerRevisionRef':
              .: {}
              'f:name': {}
            'f:kind': {}
            'f:name': {}
          'f:volumeSnapshotStatuses': {}
          'f:observedGeneration': {}
          'f:desiredGeneration': {}
      manager: virt-controller
      operation: Update
      subresource: status
      time: '2026-01-21T14:44:21Z'
  namespace: not-exist
spec:
  dataVolumeTemplates:
    - metadata:
        name: argo-test
      spec:
        sourceRef:
          kind: DataSource
          name: rhel8
          namespace: openshift-virtualization-os-images
        storage:
          resources:
            requests:
              storage: 30Gi
          storageClassName: managed-csi
  instancetype:
    kind: virtualmachineclusterinstancetype
    name: u1.medium
  preference:
    kind: virtualmachineclusterpreference
    name: rhel.8
  runStrategy: Manual
  template:
    metadata:
      labels:
        network.kubevirt.io/headlessService: headless
    spec:
      architecture: amd64
      domain:
        cpu:
          model: Broadwell
        devices:
          autoattachPodInterface: false
          disks:
            - bootOrder: 1
              name: rootdisk
          interfaces:
            - masquerade: {}
              name: default
        machine:
          type: pc-q35-rhel9.6.0
        resources: {}
      networks:
        - name: default
          pod: {}
      subdomain: headless
      volumes:
        - dataVolume:
            name: from-sub02-rhel8-volume
          name: rootdisk
        - cloudInitNoCloud:
            userData: |
              #cloud-config
              chpasswd:
                expire: false
              password: qi9w-2pg7-ooxn
              user: rhel
          name: cloudinitdisk
