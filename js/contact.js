jQuery(document).ready(function(){
	// JavaScript Starts Here //
	$("#contactForm").submit(function(){
		event.preventDefault();
		var name = $("#name").val();
		var email = $("#email").val();
		var message = $("#message").val();
		if (name.length <= 1) {
			alert("The name field is too short.");
			return false;
		} else if (!/^([a-zA-Z0-9_.+-])+\@(([a-zA-Z0-9-])+\.)+([a-zA-Z0-9]{2,4})+$/.test(email)) {
			alert("Your email seems incorrect.");
			return false;
		} else if (message.length < 20) {
			alert("Please write a message with 20 or more characters.")
			return false;
		}

		$.post("/sendMessage", {
			name: name,
			email: email,
			message: message
		}, function(result){
			alert(result);
			if (result == "OK") {
				alert("Message sent successfully!");
			} else {
				alert("An error occured when trying to send your message.");
			}
		}).fail(function(){
			alert("An error occured when trying to send your message.")
		});
	});
});