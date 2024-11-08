# workshop-builder
Toolkit for creating student environments for workshops

The goal of this project is to make it easy to spin up environments for use in workshops at conferences or other similar environments. 

This project spawned from a number of great workshops that were essentially cut short because there was so much overhead in getting the student environments created and dealing with different connectivity issues. 

## Project Status
The prototype has been completed and is at a base level functional. On a basic level it is able to create new environments on the fly and is able to push things forward to be able to monitor the status of the students in a grafana dashboard. While it is functional many of the code changes that have been made have been quick and dirty. There is a bit of work that is needed to go through everything and get it to where it is a more production ready project. Roadmap for what this looks like is coming soon.

## Current Features
 - This readme right here is currently the only production ready part of this project. 

## Current Prototype Features (Proof of concept level robustness)
 - Initial services setup can be done with the setup.sh shell script.
 - The commands.sh shell script takes one argument to create a new namespace/student environment.
   - Student environments consist of a server and a wetty interface for the student to interface with the server.
 - Run shell-tutor as a test application
 - Import the shell-tutor progress logs into Loki for processing
 - Show state in Grafana dashboard for instructor.
 - Show state to student by modifying the prompt. (shell-tutor)
 - Incoorperate state management with persistent volumes.

## Planned features
 - Easily provide student environments where all of the necessary tools are pre-installed; no wasted time on creating accounts, compiling or installing extra packages.  They just log in and go.
 - Student web interface with main workshop outline.
 - Integrated progress tracking based on environment state or files etc.
 - Shell based guidance system to walk the student through the outline without the need to be going back to the web interface. This can also provide helpers to be able to nudge students in the correct direction if needed. 
 - Auto advance button that will automatically perform the current needed step if needed to catch up with the class. 
 - Easy deployment script for students to be able to retry the workshop after the fact in case they want to revisit it. 
 - State management/download. Be able to download state and resume later/after the workshop in your own environment. 
 - Instructor interface with global tracking all students to be able to know if people are keeping up. 
