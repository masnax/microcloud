#!/bin/sh -eu


. /.wt/jp/test/includes/microcloud.sh
. /.wt/jp/test/suites/basic.sh
. /.wt/jp/test/suites/add.sh
. /.wt/jp/test/suites/preseed.sh
. /.wt/jp/test/suites/recover.sh

#!/bin/sh -eu

TEST_RESULT="fail"
TEST_CURRENT=0

cleanup() {
 echo "cleaning up ${TEST_RESULT}: ${TEST_CURRENT}"
}

func() {
 lxc remote switch local || true
 lxc project switch microcloud-test || true

 #  the VMs if you want fresh VMS, otherwise comment out.

 if [ "${1}" = "clean" ]; then
   cleanup_systems || true
 fi

 # If 1 Skip logs during setup
 SKIP_SETUP_LOG=0

 # If 1 Run the setup concurrenctly
 CONCURRENT_SETUP=1

 # If 1, Use snapshot restore
 SNAPSHOT_RESTORE=1

 # Debug binary paths on the host
 MICROCEPH_SNAP_CHANNEL="latest/edge"
 MICROOVN_SNAP_CHANNEL="22.03/stable"
 MICROOVN_SNAP_PATH="/root/go/src/github.com/canonical/microovn/microovn.snap"
 MICROCLOUD_SNAP_CHANNEL="latest/edge"
 LXD_SNAP_CHANNEL="latest/edge"
 MICROCLOUD_SNAP_PATH=""
 MICROCLOUD_DEBUG_PATH="/root/go/bin/microcloud"
 MICROCLOUDD_DEBUG_PATH="/root/go/bin/microcloudd"
 LXD_DEBUG_PATH=""
 MICROOVN_SNAP_PATH=""
 MICROOVN_SNAP_CHANNEL="latest/edge"
 MICROCEPH_SNAP_PATH="/root/go/src/github.com/canonical/microceph/microceph.snap"
# LXD_DEBUG_PATH="/root/go/bin/lxd"

 # Create VMs if they don't exist, otherwise comment out.
 if [ "${1}" = "clean" ]; then
   new_systems 4 3 3
 elif [ "${1}" = "reset" ]; then
   reset_systems 4 3 3
 fi


 #reset_systems 3 3 3

 export SKIP_VM_LAUNCH=1
 set -eux
 TEST_CURRENT=1
 #test_non_ha
 TEST_CURRENT=2
 #test_preseed
 TEST_CURRENT=3
 #test_service_mismatch
 TEST_CURRENT=4
 #test_disk_mismatch
 TEST_CURRENT=5
 #test_interactive
 TEST_CURRENT=6
 #test_recover
 TEST_CURRENT=7
 #test_reuse_cluster
 TEST_CURRENT=8
 #test_auto
 TEST_CURRENT=9
 #test_remove_cluster_member
 TEST_CURRENT=10
 #test_instances_config
 TEST_CURRENT=11
 #test_instances_launch
 TEST_CURRENT=12
 #test_add_interactive
 TEST_CURRENT=13
 #test_add_auto
 TEST_CURRENT=14
 #test_interactive_combinations
 TEST_CURRENT=15
 #test_add_services
 TEST_CURRENT=16
 return 0
}

trap cleanup EXIT HUP INT TERM

func "${@}"
TEST_RESULT="success"



 #MICROCLOUDD_DEBUG_PATH="/root/go/bin/microcloudd"
 #MICROCLOUD_DEBUG_PATH="/root/go/bin/microcloud"
 #LXD_DEBUG_PATH="/root/go/bin/lxd"
