- selected agent should be default for the tui, saved in the config db

- add a shortcut (ctrl+m) to open the agent menu in the tui directly and select a new agent

- convert this 'assistant' to a modern agent that can be customized with the personas. it should have a execution loop that can run tools and respond to the user in a conversational way. it should also be able to ask the user for more information if needed, but only in the terminal. 

- add more tools:
   - file listing (ignore hidden and dotfiles)
   - file content search (ignore hidden and dotfiles)
   - file content read (ignore hidden and dotfiles)
   - read system information (cpu, memory, disk usage)
   - read network information (ip address, open ports)
   - read process information (running processes, resource usage)
   - execute shell commands (with confirmation)
   - write to /tmp/term-ai/ folder (and subfolders)
   - execute scripts in /tmp/term-ai/ folder (with confirmation) 


- the cli version should have a fixed persona, called 'term', that is focused on terminal interactions and has access to the tools mentioned above. it should be able to understand user commands and execute them using the tools, while also providing feedback and asking for confirmation when necessary. it should be focussed on short, concise answers, and should try to execute commands that the user is trying to run. 
