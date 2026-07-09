# This is a bash shell fragment -*- bash -*-
load "${ABS_TOP_TEST_DIRNAME}test_helper/setup_teardown/$(basename "${BATS_TEST_FILENAME//.bats/.bash}")"

setup() {
    load ${ABS_TOP_TEST_DIRNAME}test_helper/common.bash
    load ${ABS_TOP_TEST_DIRNAME}test_helper/microovn.bash
    load ${ABS_TOP_TEST_DIRNAME}test_helper/lxd.bash
    load ${ABS_TOP_TEST_DIRNAME}test_helper/bgp_utils.bash

    load ${ABS_TOP_TEST_DIRNAME}../.bats/bats-support/load.bash
    load ${ABS_TOP_TEST_DIRNAME}../.bats/bats-assert/load.bash

    # Ensure required environment variables are set, otherwise the tests below will
    # provide false positive results.
    assert [ -n "$TEST_CONTAINERS" ]
    assert [ -n "$BGP_PEER" ]
}

teardown() {
    print_diagnostics_on_failure $TEST_CONTAINERS
}

bgp_data_plane_register_test_functions() {
    bats_test_function \
        --description "Test microovn setup bgp fails on no bridge or interface" \
        -- no_br_or_int
    bats_test_function \
        --description "Test microovn setup bgp fails on both bridge and interface" \
        -- br_and_int
    bats_test_function \
        --description "Test microovn setup bgp fails on non-existant interface" \
        -- setup_bgp_bad_interface
    bats_test_function \
        --description "Test microovn setup bgp propagates in cluster" \
        -- setup_bgp_established
    bats_test_function \
        --description "Test microovn join fails on node without interface" \
        -- cluster_join_bgp_no_interface
    bats_test_function \
        --description "Test microovn join succeeds on node with interface" \
        -- cluster_join_bgp_with_interface


}

function no_br_or_int() {
    run lxc_exec "$LEADER" "microovn setup bgp"
    assert_failure
}

function br_and_int() {
    run lxc_exec "$LEADER" "microovn setup bgp \
        --interface eth413 \
        --br cool_br0"
    assert_failure

}

function setup_bgp_bad_interface() {
    run lxc_exec "$LEADER" "microovn setup bgp \
        --interface eth612"
    assert_failure
}

function setup_bgp_established() {
    read -ra containers <<< "$TEST_CONTAINERS"
    # Start FRR in BGP peer container
    local tor_asn=4200000100
    echo "# Starting BGP in $BGP_PEER on interface $BGP_CONTAINER_INT_IFACE" >&3
    frr_start_bgp_unnumbered "$BGP_PEER" "$tor_asn" "eth1" "eth2" "eth3" "eth4"

    # Enable BGP redirection and start BGP daemon in OVN chassis
    local external_connections="$OVN_CONTAINER_INT_IFACE"

    echo "# Enabling MicroOVN BGP in ${containers[0]} and configuring BGP" >&3
    lxc_exec "$LEADER" "microovn setup bgp \
        --interface $external_connections"


    echo "# ${containers[0]} waiting on established BGP with $BGP_PEER" >&3
    wait_until "microovn_bgp_established ${containers[0]} $BGP_PEER"
    echo "# ${containers[1]} waiting on established BGP with $BGP_PEER" >&3
    wait_until "microovn_bgp_established ${containers[1]} $BGP_PEER"

}

function cluster_join_bgp_no_interface() {
    read -ra containers <<< "$TEST_CONTAINERS"
    JOIN_TOKEN="$(microovn_cluster_get_join_token "$LEADER" "${containers[2]}")"
    run lxc_exec "${containers[2]}" "microovn cluster join $JOIN_TOKEN"
    assert_failure
}

function cluster_join_bgp_with_interface() {
    read -ra containers <<< "$TEST_CONTAINERS"
    connect_container_to_network_no_ip "${containers[3]}" "$BGP_INT_NET-4" "$OVN_CONTAINER_INT_IFACE"
    JOIN_TOKEN="$(microovn_cluster_get_join_token "$LEADER" "${containers[3]}")"

    run lxc_exec "${containers[3]}" "microovn cluster join $JOIN_TOKEN"
    assert_success
    echo "# ${containers[3]} waiting on established BGP with $BGP_PEER" >&3
    wait_until "microovn_bgp_established ${containers[3]} $BGP_PEER"

}
bgp_data_plane_register_test_functions
