$('#all-tasks tr').click(function (event) {
	if($(this).find(".expand").val() == 0){
		$(this).find(".expand").text("Collapse");
		$(this).find(".expand").val(1);
	} else {
		$(this).find(".expand").text("Expand");
		$(this).find(".expand").val(0);
	}
 });