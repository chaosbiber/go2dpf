# Go to dpf

## Fork from plumbum

Kudos to the original author [plumbum/go2dpf](https://github.com/plumbum/go2dpf).

I had deprecation warnings on older linux distros and trouble building it on recent go versions, caused by the used libusb wrapper. Lacking libusb experience I tried having ChatGPT to rewrite the code for google/gousb and it just worked (for me). No guarantees.

Meanwhile dpf-ax was also [forked to github](https://github.com/dreamlayers/dpf-ax).

## Original README below

**Work in progress**

go2dpf allows you to record graphics in [hacked photo frames](https://sourceforge.net/projects/dpf-ax/).

For usb use [Go wrapper](https://github.com/deadsy/libusb) around the [libusb](http://www.linux-usb.org/).

Used photo frame based on microcontroller [Appotech AX206](http://picframe.spritesserver.nl/wiki/index.php/DPF_with_AppoTech_AX206).
