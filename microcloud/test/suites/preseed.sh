
test_preseed() {
  reset_systems 4 3 2
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)

  lxc exec micro01 -- sh -c "
  cat << EOF > /root/preseed.yaml
lookup_subnet: ${addr}/24
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
- name: micro02
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/sdc
      wipe: true
    ceph:
      - path: /dev/sdb
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro03
  ovn_uplink_interface: enp6s0

ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64

storage:
  local:
    - find: id == sdb
      find_min: 2
      find_max: 2
      wipe: true
  ceph:
    - find: id == sdc
      find_min: 2
      find_max: 2
      wipe: true
    - find: id == sdd
      find_min: 2
      find_max: 2
      wipe: true
EOF
"

  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --preseed /root/preseed.yaml"

  for m in micro01 micro03 ; do
    validate_system_lxd ${m} 3 disk1 2 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    validate_system_microceph ${m} disk2 disk3
    validate_system_microovn ${m}
  done

  # Disks on micro02 should have been manually selected.
  validate_system_lxd micro02 3 sdc 2 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
  validate_system_microceph micro02 disk1 disk3
  validate_system_microovn micro02

  lxc exec micro01 -- sh -c "
  cat << EOF > /root/preseed.yaml
lookup_subnet: ${addr}/24
systems:
- name: micro04
  ovn_uplink_interface: enp6s0
storage:
  local:
    - find: id == sdb
      find_min: 1
      find_max: 1
      wipe: true
  ceph:
    - find: id == sdc
      find_min: 1
      find_max: 1
      wipe: true
EOF
"

  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud add --preseed /root/preseed.yaml"
  validate_system_lxd micro04 4 disk1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
  validate_system_microceph micro04 disk2
  validate_system_microovn micro04
}