function validate_fields() {
    if($("#tags").val().trim() != "" && $("#path").val().trim() != "" && $("#description").val().trim() != ""){
        $("#add-btn").prop('disabled', false);
    } else {
        $("#add-btn").prop('disabled', true);
    }
    return;
}
$("#path").change(function() {
    validate_fields();
});
$("#description").change(function() {
    validate_fields();
});
$("#tags").change(function() {
    validate_fields();
});
$("#add-btn").click(function() {
    validate_fields();
});
