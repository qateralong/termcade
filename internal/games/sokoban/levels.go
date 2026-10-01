package sokoban

// Levels in the usual Sokoban notation:
//
//	#  wall    .  goal    $  box    *  box on a goal
//	@  you     +  you on a goal     (space) floor
//
// Every level is checked for solvability by the tests.
var levels = []struct {
	name string
	rows []string
}{
	{"First Push", []string{
		"#######",
		"#     #",
		"# @$ .#",
		"#     #",
		"#######",
	}},
	{"Two by Two", []string{
		"########",
		"#      #",
		"# .$$. #",
		"#      #",
		"#  @   #",
		"########",
	}},
	{"Side Step", []string{
		"#######",
		"#.  # #",
		"#.$ $ #",
		"#  @  #",
		"#######",
	}},
	{"Corridor", []string{
		"########",
		"#   .  #",
		"# #$#  #",
		"#   $ @#",
		"# .    #",
		"########",
	}},
	{"Crossroads", []string{
		"#########",
		"#   #   #",
		"# $ . $ #",
		"#  .@.  #",
		"# $ . $ #",
		"#   #   #",
		"#########",
	}},
	{"Hook", []string{
		" ######",
		" #    #",
		"##$## #",
		"#  .  #",
		"# #.$ ##",
		"#  @   #",
		"########",
	}},
	{"Pillars", []string{
		"#########",
		"#   .   #",
		"# $ # $ #",
		"#.@ #  .#",
		"# $ # $ #",
		"#   .   #",
		"#########",
	}},
	{"Back Room", []string{
		"  ######",
		"  #    #",
		"### ## #",
		"# $ $  #",
		"#  .  .#",
		"#  @   #",
		"########",
	}},
	{"Shuffle", []string{
		"########",
		"#  ..  #",
		"# $$$$ #",
		"#  ..  #",
		"#  @   #",
		"########",
	}},
	{"Warehouse", []string{
		"##########",
		"#   #    #",
		"# $   $  #",
		"## ## ## #",
		"# $...   #",
		"#    #  @#",
		"##########",
	}},
}
