// Never reaches an action: the bot misses every beat (IDLE) and its code keeps spinning.
function loop() {
  while (1) {
    num[0] = num[0] + 1;
  }
}
