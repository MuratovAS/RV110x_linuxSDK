#!/bin/bash

# Remove test binaries from OEM before packaging
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/librkcrypto_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_adc_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_event_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_gpio_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_led_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_pwm_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_system_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_time_test
rm -f $RK_PROJECT_PACKAGE_OEM_DIR/usr/bin/rk_watchdog_test
