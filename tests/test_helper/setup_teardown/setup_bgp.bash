# setup_file
#
# This functions sets up a simple topology for testing data-plane
# connectivity between external networks and OVN networks advertised
# via BGP.
#
#  +------------------+                +------------------+                +------------------+
#  |     Ext. Host    |     EXT_NET    |  TOR (BGP peer)  |    INT_NET     |    OVN Chassis   |
#  |             eth1 ------------------eth2          eth1------------------eth1              |
#  +------------------+                +------------------+                +------------------+
#
setup_file() {
    load test_helper/common.bash
    load test_helper/lxd.bash
    load test_helper/microovn.bash
    load test_helper/bgp_utils.bash

    TEST_CONTAINERS=$(container_names "$BATS_TEST_FILENAME" 4)
    export TEST_CONTAINERS

    launch_containers_args "-c linux.kernel_modules=vrf,openvswitch -c security.nesting=true" $TEST_CONTAINERS
    wait_containers_ready $TEST_CONTAINERS
    install_microovn "$MICROOVN_SNAP_PATH" $TEST_CONTAINERS
    setup_snap_aliases $TEST_CONTAINERS
    read -ra containers <<< "$TEST_CONTAINERS"
    bootstrap_cluster "${containers[0]}" "${containers[1]}"
    LEADER="${containers[0]}"

    # Setup networks between MicroOVN chassis, BGP peer and external host
    BGP_INT_NET="ovn-bgp-net"

    # Launch BGP peer container
    BGP_PEER="microovn-bgp-peer"
    launch_containers "$BGP_PEER"
    wait_containers_ready "$BGP_PEER"

    # Connect containers via LXD networks
    OVN_CONTAINER_INT_IFACE="eth1"

    local i=1
    for container in $TEST_CONTAINERS; do
        create_lxd_network_no_dhcp "$BGP_INT_NET-$i"
        connect_container_to_network_no_ip "$BGP_PEER" "$BGP_INT_NET-$i" "eth$i"
        if [ $i -le 2 ]; then
            connect_container_to_network_no_ip "$container" "$BGP_INT_NET-$i" "$OVN_CONTAINER_INT_IFACE"
        fi
        i=$((++i))
    done


    # Install FRR in peer containers
    install_frr_bgp $BGP_PEER

    # Export test-related variables
    export BGP_PEER
    export TEST_CONTAINERS
    export OVN_CONTAINER_INT_IFACE
    export BGP_INT_NET
    export LEADER
}


teardown_file() {
    print_diagnostics_on_failure $TEST_CONTAINERS
    collect_coverage $TEST_CONTAINERS
    delete_containers "$TEST_CONTAINERS $BGP_PEER"
    for i in {1..4}; do
        delete_lxd_network "$BGP_INT_NET-$i"
        i=$((++i))
    done
}
