function validate_fields() {
    if($("#tags").val().trim() != "" && $("#path").val().trim() != "" && $("#description").val().trim() != ""){
        $("#add-btn").prop('disabled', false);
    } else {
        $("#add-btn").prop('disabled', true);
    }
    return;
}
$("#path").on('input', function() {
    validate_fields();
});
$("#description").on('input', function() {
    validate_fields();
});
$("#tags").on('input', function() {
    validate_fields();
});
$("#add-btn").click(function() {
    validate_fields();
});
