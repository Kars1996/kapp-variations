use std::fs;
use std::io::{self, Write};
use std::process::Command;
use std::path::Path;
use std::thread::sleep;
use std::time::Duration;
use std::error::Error;

use reqwest;
use zip::ZipArchive;

enum Completed {
    Confirm,
    Input,
}

struct KPrompts {
    colors: Colors,
}

impl KPrompts {
    fn new() -> Self {
        Self::fix_colors();
        Self {
            colors: Colors {
                cyan: "\x1b[0;96m".to_string(),
                green: "\x1b[0;92m".to_string(),
                red: "\x1b[0;91m".to_string(),
                white: "\x1b[0;97m".to_string(),
                grey: "\x1b[1;30m".to_string(),
            },
        }
    }

    fn fix_colors() {
        if cfg!(windows) {
            let output = Command::new("cmd")
                .arg("/C")
                .arg("color")
                .output()
                .expect("Failed to set console color mode");
            assert!(output.status.success());
        }
    }

    fn print(&self, text: &str) {
        print!("{}{}", self.colors.white, text);
        io::stdout().flush().unwrap();
    }

    fn final_print(&self, question: &str, answer: &str, password: bool) {
        println!(
            "{}√{} {}{} » {}",
            self.colors.green, self.colors.white, question, self.colors.grey, answer
        );
    }

    fn better_input(&self, text: &str) -> String {
        self.print(&format!("{}? {}{}", self.colors.cyan, self.colors.white, text));
        print!(" {} » ", self.colors.grey);
        io::stdout().flush().unwrap();

        let mut user_input = String::new();
        io::stdin().read_line(&mut user_input).unwrap();
        print!("\x1b[F\x1b[K"); // Clears previous input

        user_input.trim().to_string()
    }

    fn prompt(&self, option: Completed, message: &str, validate: Option<Box<dyn Fn(&str) -> bool>>, keep: Option<bool>) -> String {
        match option {
            Completed::Input => {
                loop {
                    let user_input = self.better_input(message);
                    if let Some(validate_fn) = &validate {
                        if validate_fn(&user_input) {
                            if keep.unwrap_or(true) {
                                self.final_print(message, &user_input, false);
                            }
                            return user_input;
                        } else {
                            self.print(&format!("{}× Invalid input. Try again.{}", self.colors.red, self.colors.white));
                        }
                    } else {
                        if keep.unwrap_or(true) {
                            self.final_print(message, &user_input, false);
                        }
                        return user_input;
                    }
                }
            }
            Completed::Confirm => {
                loop {
                    let user_input = self.better_input(&format!("{} (y/n)", message)).to_lowercase();
                    match user_input.as_str() {
                        "y" | "n" => {
                            if keep.unwrap_or(true) {
                                self.final_print(message, &user_input, false);
                            }
                            return user_input;
                        }
                        _ => self.print(&format!("{}× Please answer with 'y' or 'n'.{}", self.colors.red, self.colors.white)),
                    }
                }
            }
        }
    }
}

struct CreateKapp {
    user: String,
    branch: String,
    urls: Vec<&'static str>,
    colors: Colors,
    prompt: KPrompts,
}

impl CreateKapp {
    fn new(user: String, branch: String) -> Self {
        Self {
            user,
            branch,
            urls: vec!["template", "apitemplate", "DJS14Template"],
            colors: Colors {
                cyan: "\x1b[0;96m".to_string(),
                green: "\x1b[0;92m".to_string(),
                red: "\x1b[0;91m".to_string(),
                white: "\x1b[0;97m".to_string(),
                grey: "\x1b[1;30m".to_string(),
            },
            prompt: KPrompts::new(),
        }
    }

    fn set_path(&self, path: &str) -> Result<String, Box<dyn Error>> {
        let found_path = if path == "." {
            fs::canonicalize(".")?.to_str().unwrap().to_string()
        } else {
            if !Path::new(path).exists() {
                fs::create_dir(path)?;
            }
            fs::canonicalize(path)?.to_str().unwrap().to_string()
        };
        Ok(found_path)
    }

    async fn download(&self, url: &str, found_path: &str) -> Result<(), Box<dyn Error>> {
        let download_url = format!(
            "https://github.com/{}/archive/refs/heads/{}.zip",
            self.user, self.branch
        );
        self.prompt.print(&format!(
            "{}∂ Downloading template {}...{}",
            self.colors.cyan, url, self.colors.white
        ));

        let response = reqwest::get(&download_url).await?;
        let mut archive = ZipArchive::new(io::Cursor::new(response.bytes().await?))?;

        archive.extract(found_path)?;

        self.answer("Download and extraction complete!", true);
        Ok(())
    }

    fn run(&self) -> Result<(), Box<dyn Error>> {
        let folder = self.prompt.prompt(
            Completed::Input,
            "Setup the project in (specify folder)...?",
            Some(Box::new(|value| value.len() > 0)),
            Some(true),
        );

        let found_path = self.set_path(&folder)?;

        let scaffold = self.prompt.prompt(
            Completed::Input,
            "What scaffold do you want to start with?",
            None,
            Some(true),
        );
        let scaffold = if self.urls.contains(&scaffold.as_str()) {
            scaffold
        } else {
            self.urls[0].to_string()
        };

        self.download(&scaffold, &found_path).unwrap();
        self.answer("Successfully set up project :D", true);
        Ok(())
    }

    fn answer(&self, message: &str, success: bool) {
        if success {
            println!("{}{}", self.colors.green, message);
        } else {
            println!("{}{}", self.colors.red, message);
        }
    }
}

struct Colors {
    cyan: String,
    green: String,
    red: String,
    white: String,
    grey: String,
}

fn main() {
    let app = CreateKapp::new("kars1996".to_string(), "master".to_string());
    app.run().unwrap();
}
